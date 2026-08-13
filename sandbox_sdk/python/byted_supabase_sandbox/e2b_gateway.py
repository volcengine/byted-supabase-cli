"""Point the official E2B SDK at a Volcengine Supabase compute-gateway instance.

One call at startup, then everything else is stock E2B -- same imports, same API, same docs::

    from sdk_patch import init_byted_supabase_sandbox

    init_byted_supabase_sandbox(url="https://<branch-host>", api_key="<supabase jwt>")

    from e2b import Sandbox
    sbx = Sandbox.create("base")

To make that work we patch the official classes in place. What gets patched is deliberately small:

* **three URL builders** -- ``get_host`` / ``_file_url`` / ``_jupyter_url``. The gateway addresses
  port previews, signed file URLs and the Jupyter channel by path, not by e2b's
  ``{port}-{id}.{domain}`` subdomain scheme. These are the *least* volatile methods in the SDK
  (unchanged for a year), which is why this is a safe place to intervene.
* **thin wrappers on the control-plane entry points** -- to supply connection options and to retry
  what the gateway marked retryable.

What is deliberately **not** patched, and why it matters:

* ``fetch`` / ``undici`` / ``httpx`` are untouched. An earlier version intercepted both Node fetch
  targets to attach a credential to every request -- including data-plane ones. That was never
  necessary: the gateway does not read a platform JWT on the data plane at all, it authorises by
  possession of the unguessable ``sandbox_id``, precisely because the SDKs hardcode their envd
  headers. Removing that interception also removes its module-snapshot ordering hazard.
* ``ConnectionConfig.__init__`` is untouched. Every setting we need (``api_url``, ``sandbox_url``,
  ``api_key``, ``validate_api_key``, ``request_timeout``) is an official ``ApiParams`` field, so it
  is passed as an argument rather than injected by rewriting a constructor that upstream adds
  parameters to several times a year.

The wrappers never inspect the credential and never second-guess the request. Sandbox ownership and
the rules around it are the gateway's business, so arguments are passed through and the gateway's
own errors are what the caller sees. Duplicating those rules here would give two sources of truth
that drift, and would leave an old client rejecting requests a newer gateway considers valid.

Patching the class object itself means import order does not matter: a module that did
``from e2b import Sandbox`` before the call holds the very same object we patch.
"""

from __future__ import annotations

import asyncio
import inspect
import os
import re
import time
from contextlib import contextmanager
from typing import Any, Callable, Mapping, Optional, Union
from urllib.parse import quote, urlencode

import httpx
from e2b.exceptions import AuthenticationException, RateLimitException, SandboxException

__all__ = [
    "init_byted_supabase_sandbox",
    "GatewayPatch",
    "active_patch",
    "normalize_gateway_url",
    "GATEWAY_API_SUFFIX",
    "SandboxGatewayError",
    "SandboxAuthError",
    "SandboxPermissionError",
    "SandboxNotFoundError",
    "SandboxInvalidArgument",
    "SandboxStateConflict",
    "SandboxRateLimited",
    "SandboxQuotaExceeded",
    "SandboxThrottled",
    "SandboxNotSupported",
    "SandboxUpstreamError",
]


# ============================================================================================
# Gateway URL
# ============================================================================================

#: Every request the SDK makes ends up under this prefix. Fixed by the gateway, not configurable.
GATEWAY_API_SUFFIX = "/sandbox/v1"

_SCHEME_RE = re.compile(r"^[a-z][a-z0-9+.-]*://", re.IGNORECASE)
# Absorb a suffix the caller already supplied so normalisation is idempotent. Longest first:
# `/sandbox/v1` before `/sandbox`, and a bare `/v1` for people who guessed at versioning.
_TRAILING_SUFFIX_RE = re.compile(r"(?:/sandbox/v1|/sandbox|/v1)$", re.IGNORECASE)


def normalize_gateway_url(raw: str | None) -> str:
    """Return ``raw`` rewritten to end in :data:`GATEWAY_API_SUFFIX`.

    Accepts a bare host, a branch URL, or something already ending in ``/sandbox`` or
    ``/sandbox/v1``. A sub-path prefix is preserved -- when Kong mounts the gateway under ``/proj``
    the result is ``https://host/proj/sandbox/v1``; only the trailing segment is rewritten.

    Raises ``ValueError`` on an empty URL. We never fall back to the public e2b.app: a deployment
    that silently talks to the wrong backend is far worse than one that fails at startup.
    """
    value = "" if raw is None else str(raw).strip()
    if not value:
        raise ValueError(
            "no gateway url: pass url=..., or set the E2B_API_URL / E2B_DOMAIN environment variable"
        )
    if not _SCHEME_RE.match(value):
        value = "https://" + value
    value = value.rstrip("/")
    value = _TRAILING_SUFFIX_RE.sub("", value)
    return value + GATEWAY_API_SUFFIX


# ============================================================================================
# Errors -- a rename of the gateway's HTTP status, never an interpretation of its message
# ============================================================================================


class SandboxGatewayError(SandboxException):
    """Base class for gateway-classified errors.

    Subclasses ``e2b.exceptions.SandboxException`` on purpose: an ``except SandboxException``
    written against the official SDK still catches these.
    """

    def __init__(self, message: str, *, status: int | None = None) -> None:
        super().__init__(message)
        self.message = message
        self.status = status


class SandboxAuthError(SandboxGatewayError):
    """401 -- missing, malformed, or expired credential."""


class SandboxPermissionError(SandboxGatewayError):
    """403 -- authenticated, but not allowed to do this."""


class SandboxNotFoundError(SandboxGatewayError):
    """404 -- no such sandbox, template, or endpoint."""


class SandboxInvalidArgument(SandboxGatewayError):
    """400 -- the request was rejected as malformed."""


class SandboxStateConflict(SandboxGatewayError):
    """409 -- the sandbox is not in a state that permits this operation."""


class SandboxRateLimited(SandboxGatewayError):
    """Catch this to handle both exhausted quota and transient backpressure at once."""


class SandboxQuotaExceeded(SandboxRateLimited):
    """429 -- quota exhausted. Retrying will not help until the window rolls over."""


class SandboxThrottled(SandboxRateLimited):
    """503 -- transient backpressure, or the sandbox is not ready yet. Retrying should work.

    Covers the instance rate limiter, the concurrent-create gate, and a create/resume that did not
    become ready inside the gateway's wait window.
    """


class SandboxNotSupported(SandboxGatewayError):
    """501 -- the capability is not offered by this compute backend.

    Distinct from 404: the endpoint exists in the e2b contract, this deployment just does not
    implement it (``pause``/``resume`` are the common ones). When the gateway rejected specific
    create options it names them in the message.
    """


class SandboxUpstreamError(SandboxGatewayError):
    """502 -- the compute backend failed or is unreachable."""


# The official SDK formats every API error as "<status>: <body message>"; 401 and 429 additionally
# get a canned sentence, with the gateway's own message appended after " - ".
# See e2b/api/__init__.py::handle_api_exception.
_STATUS_RE = re.compile(r"^(\d{3}):\s*(.*)$", re.DOTALL)

# Status alone decides the type -- the message is never inspected.
#
# The gateway distinguishes "retry will not help" (429, quota exhausted) from "retry almost
# certainly will" (503, transient backpressure or not-yet-ready) in the status code itself, so
# there is nothing left to infer from the wording. That matters because message text is the one
# part of an error contract that changes freely; anything derived from it here would be a second
# source of truth that silently drifts from the gateway's.
_BY_STATUS: dict[int, type[SandboxGatewayError]] = {
    400: SandboxInvalidArgument,
    401: SandboxAuthError,
    403: SandboxPermissionError,
    404: SandboxNotFoundError,
    409: SandboxStateConflict,
    429: SandboxQuotaExceeded,
    501: SandboxNotSupported,
    502: SandboxUpstreamError,
    503: SandboxThrottled,
}


def _split_status(exc: BaseException) -> tuple[int | None, str]:
    text = str(exc)
    match = _STATUS_RE.match(text)
    if not match:
        return None, text
    status, rest = int(match.group(1)), match.group(2)
    if isinstance(exc, (AuthenticationException, RateLimitException)) and " - " in rest:
        rest = rest.split(" - ", 1)[1]
    return status, rest


def translate(exc: BaseException) -> BaseException:
    """Return a typed :class:`SandboxGatewayError` for a recognisable SDK error, else ``exc``.

    Anything without a parseable status prefix is passed straight through -- transport failures and
    the SDK's own client-side errors are not ours to reinterpret.
    """
    status, message = _split_status(exc)
    if status is None:
        return exc
    translated = _BY_STATUS.get(status, SandboxGatewayError)(message, status=status)
    translated.__cause__ = exc
    return translated


#: Transport-level failures. The gateway blocks until the sandbox is ready (a 90-133s cold start),
#: so a client-side read timeout here says nothing about whether the request was fatal.
_RETRYABLE_TRANSPORT = (httpx.TimeoutException, httpx.ConnectError)


def is_retryable(exc: BaseException) -> bool:
    """True for what the gateway marked retryable, false for anything a retry cannot fix.

    The whole rule is "did the gateway say 503". It can be that simple because the gateway separates
    retryable conditions from exhausted quota at the status-code level, so nothing here reads a
    message. An earlier version matched a bare ``"timeout"`` substring instead, which silently swept
    in argument errors the caller had made, each costing a cold-start window to fail.
    """
    if isinstance(exc, SandboxThrottled):
        return True
    return isinstance(exc, _RETRYABLE_TRANSPORT)


# ============================================================================================
# The patch
# ============================================================================================

ApiKey = Union[str, Callable[[], Optional[str]], None]

_MISSING = object()

#: ``connect`` / ``kill`` / ``get_info`` are ``class_method_variant`` descriptors that dispatch to
#: these ``_cls_*`` twins. Wrapping the twin supplies options to the class-level form while leaving
#: the instance-method form (``sbx.kill()``, which already carries its own config) untouched.
_CLS_FORWARDS = ("_cls_connect_sandbox", "_cls_kill", "_cls_get_info")

#: Set on a class we have patched, so a re-install can detect and undo the previous one.
_MARKER = "__byted_supabase_patched__"

_active: "GatewayPatch | None" = None


@contextmanager
def _typed():
    """Re-raise SDK errors as the typed equivalents above."""
    try:
        yield
    except Exception as exc:  # noqa: BLE001 -- reclassify, never swallow
        translated = translate(exc)
        if translated is exc:
            raise
        raise translated


def _raw(target: type, name: str) -> Callable[..., Any] | None:
    """The plain function behind ``name``, unwrapping classmethod/staticmethod."""
    descriptor = inspect.getattr_static(target, name, None)
    if descriptor is None:
        return None
    return getattr(descriptor, "__func__", descriptor)


class GatewayPatch:
    """Handle for an installed patch. Returned by :func:`init_byted_supabase_sandbox`."""

    def __init__(
        self,
        *,
        api_url: str,
        api_key: ApiKey = None,
        request_timeout: float | None = None,
        create_defaults: Mapping[str, Any] | None = None,
        create_retries: int = 0,
        create_retry_backoff: float = 3.0,
    ) -> None:
        self.api_url = api_url
        self.create_retries = create_retries
        self.create_retry_backoff = create_retry_backoff
        self._api_key = api_key
        self._request_timeout = request_timeout
        self._create_defaults = dict(create_defaults or {})
        self._undo: list[tuple[type, str, Any]] = []

    # --- credential + connection options ------------------------------------------------------

    def resolve_api_key(self) -> str | None:
        """Resolve the credential, calling the supplied factory if there is one.

        Re-resolved on every control-plane entry point so a short-lived ``authenticated`` JWT can be
        refreshed without re-installing the patch.
        """
        return self._api_key() if callable(self._api_key) else self._api_key

    def connection_options(self) -> dict[str, Any]:
        """Official ``ApiParams`` for this gateway.

        Also useful directly, for the few static SDK entry points not wrapped here, e.g.
        ``Sandbox.get_metrics(sandbox_id, **patch.connection_options())``.
        """
        opts: dict[str, Any] = {
            "api_url": self.api_url,
            # The control plane and envd share one prefix; envd requests are recognised by the
            # E2b-Sandbox-Id header the SDK always sends.
            "sandbox_url": self.api_url,
            # The credential is a Supabase JWT, which does not match e2b's `e2b_<hex>` format.
            "validate_api_key": False,
        }
        key = self.resolve_api_key()
        if key:
            opts["api_key"] = key
        if self._request_timeout is not None:
            opts["request_timeout"] = self._request_timeout
        return opts

    def _inject(self, kwargs: dict[str, Any]) -> dict[str, Any]:
        """Fill in connection options, letting anything the caller passed explicitly win."""
        opts = self.connection_options()
        for key, value in opts.items():
            kwargs.setdefault(key, value)
        return opts

    def _apply_create_defaults(self, kwargs: dict[str, Any]) -> None:
        """Supply default ``create`` arguments. Anything the caller passed always wins.

        Defaults only -- this fills in arguments, it never inspects or rejects them.
        """
        for key, value in self._create_defaults.items():
            if key == "metadata":
                # Merge rather than replace, so a per-call metadata entry does not drop the defaults.
                kwargs["metadata"] = {**dict(value or {}), **dict(kwargs.get("metadata") or {})}
            else:
                kwargs.setdefault(key, value)

    # --- URL builders -------------------------------------------------------------------------

    def preview_url(self, sandbox_id: str, port: int) -> str:
        """Public URL for a port inside the sandbox.

        The trailing slash is load-bearing: without it a served page's relative sub-resources
        resolve against the parent path and drop the ``{port}`` segment. The gateway 301s to add it.
        """
        return f"{self.api_url}/proxy/{quote(str(sandbox_id), safe='')}/{port}/"

    def envd_files_url(
        self,
        sandbox_id: str,
        path: str | None = None,
        user: str | None = None,
        signature: str | None = None,
        signature_expiration: int | None = None,
    ) -> str:
        """URL for the path-addressed envd files channel.

        Signed URLs get opened by a browser or curl, neither of which can carry the
        ``E2b-Sandbox-Id`` header -- hence the id in the path. The signature itself is still
        computed by the official SDK; only the URL assembly differs.
        """
        url = f"{self.api_url}/envd/{quote(str(sandbox_id), safe='')}/files"
        query: dict[str, str] = {}
        if path:
            query["path"] = path
        if user:
            query["username"] = user
        if signature:
            query["signature"] = signature
        if signature_expiration:
            if signature is None:
                raise ValueError("signature_expiration requires signature to be set")
            query["signature_expiration"] = str(signature_expiration)
        if query:
            url += "?" + urlencode(query, quote_via=quote)
        return url

    def jupyter_url(self, sandbox_id: str) -> str:
        """URL for the run_code channel.

        Deliberately the dedicated ``/jupyter/`` channel rather than ``/proxy/{id}/49999/``: it
        needs no traffic-access token, and it rewrites the "no kernel listening" upstream 502 into
        a 404 whose body actually reaches the caller (the code-interpreter SDK turns any 502 into a
        misleading ``TimeoutError``).
        """
        return f"{self.api_url}/jupyter/{quote(str(sandbox_id), safe='')}"

    # --- retry --------------------------------------------------------------------------------

    def _attempts(self) -> tuple[int, float]:
        return max(0, self.create_retries) + 1, max(0.0, self.create_retry_backoff)

    def _retrying(self, call: Callable[[], Any]) -> Any:
        attempts, backoff = self._attempts()
        for attempt in range(1, attempts + 1):
            try:
                return call()
            except Exception as exc:  # noqa: BLE001
                err = translate(exc)
                if attempt >= attempts or not is_retryable(err):
                    raise err
                if backoff:
                    time.sleep(backoff)
        raise AssertionError("unreachable")  # pragma: no cover

    async def _aretrying(self, call: Callable[[], Any]) -> Any:
        attempts, backoff = self._attempts()
        for attempt in range(1, attempts + 1):
            try:
                return await call()
            except Exception as exc:  # noqa: BLE001
                err = translate(exc)
                if attempt >= attempts or not is_retryable(err):
                    raise err
                if backoff:
                    await asyncio.sleep(backoff)
        raise AssertionError("unreachable")  # pragma: no cover

    # --- install / restore --------------------------------------------------------------------

    def _set(self, target: type, name: str, value: Any) -> None:
        self._undo.append((target, name, target.__dict__.get(name, _MISSING)))
        setattr(target, name, value)

    def restore(self) -> None:
        """Undo the patch, returning the official classes to their original behaviour."""
        global _active
        for target, name, original in reversed(self._undo):
            if original is _MISSING:
                try:
                    delattr(target, name)
                except AttributeError:  # pragma: no cover -- already gone
                    pass
            else:
                setattr(target, name, original)
        self._undo.clear()
        if _active is self:
            _active = None


def _patch_urls(patch: GatewayPatch, target: type) -> None:
    def get_host(self, port: int) -> str:
        return patch.preview_url(self.sandbox_id, port)

    def get_preview_url(self, port: int) -> str:
        """Full public URL for ``port``. Alias of ``get_host`` with an unambiguous name.

        Returns a complete URL including scheme, unlike upstream e2b where ``get_host`` returns a
        bare host meant to be prefixed with ``https://`` -- a gateway address has a path component,
        which a bare host cannot express.
        """
        return patch.preview_url(self.sandbox_id, port)

    def _file_url(self, path, user=None, signature=None, signature_expiration=None) -> str:
        return patch.envd_files_url(self.sandbox_id, path, user, signature, signature_expiration)

    patch._set(target, "get_host", get_host)
    patch._set(target, "get_preview_url", get_preview_url)
    patch._set(target, "_file_url", _file_url)


def _patch_control_plane(patch: GatewayPatch, target: type, *, is_async: bool) -> None:
    original_create = _raw(target, "create")
    original_list = _raw(target, "list")
    original_forwards = {name: _raw(target, name) for name in _CLS_FORWARDS}

    if is_async:

        async def create(cls, template=None, **kwargs):
            patch._apply_create_defaults(kwargs)
            patch._inject(kwargs)
            return await patch._aretrying(lambda: original_create(cls, template, **kwargs))

        def make_forward(original):
            async def forward(cls, *args, **kwargs):
                patch._inject(kwargs)
                with _typed():
                    return await original(cls, *args, **kwargs)

            return classmethod(forward)

    else:

        def create(cls, template=None, **kwargs):
            patch._apply_create_defaults(kwargs)
            patch._inject(kwargs)
            return patch._retrying(lambda: original_create(cls, template, **kwargs))

        def make_forward(original):
            def forward(cls, *args, **kwargs):
                patch._inject(kwargs)
                with _typed():
                    return original(cls, *args, **kwargs)

            return classmethod(forward)

    def list_(*args, **kwargs):
        # Sync in both flavours: it builds a paginator, it does not perform the request.
        patch._inject(kwargs)
        with _typed():
            return original_list(*args, **kwargs)

    if original_create is not None:
        patch._set(target, "create", classmethod(create))
    if original_list is not None:
        patch._set(target, "list", staticmethod(list_))
    for name, original in original_forwards.items():
        if original is not None:
            patch._set(target, name, make_forward(original))


def _apply(patch: GatewayPatch, target: type, *, is_async: bool) -> None:
    _patch_urls(patch, target)
    _patch_control_plane(patch, target, is_async=is_async)
    patch._set(target, _MARKER, True)


def _apply_code_interpreter(patch: GatewayPatch) -> None:
    """Point the run_code channel at the gateway.

    The code-interpreter classes subclass ``e2b.Sandbox``, so they inherit everything patched
    above; the one thing they define themselves is ``_jupyter_url``.
    """
    try:
        import e2b_code_interpreter as ci
    except ImportError:
        return
    for name in ("Sandbox", "AsyncSandbox"):
        target = getattr(ci, name, None)
        if target is None:
            continue
        patch._set(
            target, "_jupyter_url", property(lambda self: patch.jupyter_url(self.sandbox_id))
        )


def init_byted_supabase_sandbox(
    url: str | None = None,
    api_key: ApiKey = None,
    *,
    request_timeout: float | None = None,
    create_defaults: Mapping[str, Any] | None = None,
    create_retries: int = 0,
    create_retry_backoff: float = 3.0,
) -> GatewayPatch:
    """Point the official E2B SDK at a Volcengine Supabase compute-gateway instance.

    Call once at startup, then use ``e2b`` exactly as its own documentation describes::

        init_byted_supabase_sandbox(url="https://<branch-host>", api_key="<supabase jwt>")

        from e2b import Sandbox
        sbx = Sandbox.create("base")

    :param url: Gateway base URL, in any shape. Falls back to ``E2B_API_URL`` then ``E2B_DOMAIN``.
    :param api_key: A Supabase JWT: a user session token, or a ``service_role`` key from a backend.
        May be a zero-argument callable, re-invoked on every control-plane call so short-lived
        tokens can be refreshed. Falls back to ``E2B_API_KEY``.
    :param request_timeout: Per-request timeout in seconds. Must exceed the gateway's own cold-start
        ceiling (5 min) or the client gives up first and reports a transport timeout instead of the
        gateway's actual error.
    :param create_defaults: Default arguments for every ``Sandbox.create``, e.g.
        ``{"secure": True, "timeout": 900, "metadata": {"ownerUserId": "u_123"}}``. Anything passed
        at the call site wins; ``metadata`` is merged rather than replaced. Defaults only -- this
        never inspects or rejects what the caller asked for.
    :param create_retries: Extra attempts when ``create`` fails with something the gateway marked
        retryable (503). Quota and argument errors are never retried. **Defaults to 0 -- retrying
        create is opt-in.** 503 covers two very different costs: the create gate sheds in ~1ms and
        is worth retrying, but "the cluster has no capacity" is only reported after the gateway
        burns its full ready-wait window (90s for a pooled template). Retrying blindly multiplies
        the caller's wait for the same answer and pushes the same multiple of futile cold starts
        back at an already-saturated cluster. Set it explicitly when your workload can absorb that.

    Returns a handle whose ``restore()`` undoes the patch. Installing again replaces the previous
    installation -- this is process-global state, so one process serves one gateway and one
    credential at a time.
    """
    global _active

    resolved = url or os.getenv("E2B_API_URL") or os.getenv("E2B_DOMAIN")
    patch = GatewayPatch(
        api_url=normalize_gateway_url(resolved),
        api_key=api_key if api_key is not None else os.getenv("E2B_API_KEY"),
        request_timeout=request_timeout,
        create_defaults=create_defaults,
        create_retries=create_retries,
        create_retry_backoff=create_retry_backoff,
    )

    if _active is not None:
        # Never stack patches: the second install would capture the first one's wrappers as its
        # "originals" and restore would only ever peel off one layer.
        _active.restore()

    from e2b import AsyncSandbox, Sandbox

    _apply(patch, Sandbox, is_async=False)
    _apply(patch, AsyncSandbox, is_async=True)
    _apply_code_interpreter(patch)

    _active = patch
    return patch


def active_patch() -> GatewayPatch | None:
    """The currently installed patch, if any."""
    return _active
