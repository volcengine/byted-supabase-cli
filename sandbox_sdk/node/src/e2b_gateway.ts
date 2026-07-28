/**
 * Point the official E2B SDK at a Volcengine Supabase compute-gateway instance.
 *
 * One call at startup, then everything else is stock E2B -- same imports, same API, same docs:
 *
 *     import { initBytedSupabaseSandbox } from './sdk_patch/e2b_gateway'
 *     initBytedSupabaseSandbox({ url: 'https://<branch-host>', apiKey: '<supabase jwt>' })
 *
 *     import { Sandbox } from 'e2b'
 *     const sbx = await Sandbox.create('base')
 *
 * To make that work we patch the official classes in place. What gets patched is deliberately small:
 *
 * - **three URL builders** -- `getHost` / `fileUrl` / `jupyterUrl`. The gateway addresses port
 *   previews, signed file URLs and the Jupyter channel by path, not by e2b's
 *   `{port}-{id}.{domain}` subdomain scheme. These are the least volatile methods in the SDK,
 *   which is why this is a safe place to intervene.
 * - **thin wrappers on the static control-plane entry points** -- to supply connection options and
 *   to retry what the gateway marked retryable.
 *
 * What is deliberately **not** patched:
 *
 * - `globalThis.fetch` and `undici.fetch` are untouched. An earlier version intercepted both in
 *   order to attach a credential to every request, including data-plane ones. That was never
 *   necessary: the gateway does not read a platform JWT on the data plane at all, it authorises by
 *   possession of the unguessable `sandbox_id`, precisely because this SDK hardcodes its envd
 *   headers and exposes no injection point. Dropping that interception also drops its
 *   module-snapshot ordering hazard (`createRequire('undici')` before the first request).
 * - `ConnectionConfig` is untouched. Every setting we need (`apiUrl`, `sandboxUrl`, `apiKey`,
 *   `validateApiKey`, `requestTimeoutMs`) is an official `ConnectionOpts` field, so it is passed as
 *   an argument rather than injected by rewriting a constructor upstream keeps extending.
 *
 * The wrappers never inspect the credential and never second-guess the request. Sandbox ownership
 * and the rules around it are the gateway's business: arguments are passed through and the
 * gateway's own errors are what the caller sees. Duplicating those rules here would give two
 * sources of truth that drift, and would leave an old client rejecting requests a newer gateway
 * considers perfectly valid.
 */

import { AuthenticationError, RateLimitError, Sandbox, SandboxError } from 'e2b'

// ==============================================================================================
// Gateway URL
// ==============================================================================================

/** Every request the SDK makes ends up under this prefix. Fixed by the gateway, not configurable. */
export const GATEWAY_API_SUFFIX = '/sandbox/v1'

const SCHEME_RE = /^[a-z][a-z0-9+.-]*:\/\//i
// Absorb a suffix the caller already supplied so normalisation is idempotent. Longest first:
// `/sandbox/v1` before `/sandbox`, and a bare `/v1` for people who guessed at versioning.
const TRAILING_SUFFIX_RE = /(?:\/sandbox\/v1|\/sandbox|\/v1)$/i

/**
 * Rewrite `raw` to end in {@link GATEWAY_API_SUFFIX}.
 *
 * Accepts a bare host, a branch URL, or something already ending in `/sandbox` or `/sandbox/v1`.
 * A sub-path prefix is preserved -- when Kong mounts the gateway under `/proj` the result is
 * `https://host/proj/sandbox/v1`; only the trailing segment is rewritten.
 *
 * Throws on an empty URL. We never fall back to the public e2b.app: a deployment that silently
 * talks to the wrong backend is far worse than one that fails at startup.
 */
export function normalizeGatewayUrl(raw: string | null | undefined): string {
  let value = String(raw ?? '').trim()
  if (!value) {
    throw new Error(
      'no gateway url: pass url, or set the E2B_API_URL / E2B_DOMAIN environment variable'
    )
  }
  if (!SCHEME_RE.test(value)) value = `https://${value}`
  value = value.replace(/\/+$/, '').replace(TRAILING_SUFFIX_RE, '')
  return `${value}${GATEWAY_API_SUFFIX}`
}

// ==============================================================================================
// Errors -- a rename of the gateway's HTTP status, never an interpretation of its message
// ==============================================================================================

/**
 * Base class for gateway-classified errors. Extends the official `SandboxError` on purpose: a
 * `catch (e) { if (e instanceof SandboxError) ... }` written against the official SDK still works.
 */
export class SandboxGatewayError extends SandboxError {
  readonly status?: number

  constructor(message: string, status?: number, options?: { cause?: unknown }) {
    super(message)
    this.name = new.target.name
    this.status = status
    if (options?.cause !== undefined) this.cause = options.cause
  }
}

/** 401 -- missing, malformed, or expired credential. */
export class SandboxAuthError extends SandboxGatewayError {}
/** 403 -- authenticated, but not allowed to do this. */
export class SandboxPermissionError extends SandboxGatewayError {}
/** 404 -- no such sandbox, template, or endpoint. */
export class SandboxNotFoundError extends SandboxGatewayError {}
/** 400 -- the request was rejected as malformed. */
export class SandboxInvalidArgument extends SandboxGatewayError {}
/** 409 -- the sandbox is not in a state that permits this operation. */
export class SandboxStateConflict extends SandboxGatewayError {}
/** Catch this to handle both exhausted quota and transient backpressure at once. */
export class SandboxRateLimited extends SandboxGatewayError {}
/** 429 -- quota exhausted. Retrying will not help until the window rolls over. */
export class SandboxQuotaExceeded extends SandboxRateLimited {}
/**
 * 503 -- transient backpressure, or the sandbox is not ready yet. Retrying should work.
 *
 * Covers the instance rate limiter, the concurrent-create gate, and a create/resume that did not
 * become ready inside the gateway's wait window.
 */
export class SandboxThrottled extends SandboxRateLimited {}
/** 502 -- the compute backend failed or is unreachable. */
export class SandboxUpstreamError extends SandboxGatewayError {}
/**
 * 501 -- the capability is not offered by this compute backend.
 *
 * Distinct from 404: the endpoint exists in the e2b contract, this deployment just does not
 * implement it (`pause`/`resume` are the common ones). When the gateway rejected specific create
 * options it names them in the message.
 */
export class SandboxNotSupported extends SandboxGatewayError {}

// The official SDK formats most API errors as "<status>: <body message>". 401 and 429 are special
// cased with a canned sentence and *no* status prefix, so those are recognised by class instead.
// See e2b/src/api/index.ts::handleApiError.
const STATUS_RE = /^(\d{3}):\s*([\s\S]*)$/

type GatewayErrorClass = new (
  message: string,
  status?: number,
  options?: { cause?: unknown }
) => SandboxGatewayError

// Status alone decides the type -- the message is never inspected.
//
// The gateway distinguishes "retry will not help" (429, quota exhausted) from "retry almost
// certainly will" (503, transient backpressure or not-yet-ready) in the status code itself, so
// there is nothing left to infer from the wording. That matters because message text is the one
// part of an error contract that changes freely; anything derived from it here would be a second
// source of truth that silently drifts from the gateway's.
const BY_STATUS: Record<number, GatewayErrorClass> = {
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

function splitStatus(err: unknown): { status?: number; message: string } {
  const text = err instanceof Error ? err.message : String(err)
  const afterDash = () => (text.includes(' - ') ? text.split(' - ').slice(1).join(' - ') : text)
  if (err instanceof AuthenticationError) return { status: 401, message: afterDash() }
  if (err instanceof RateLimitError) return { status: 429, message: afterDash() }
  const match = STATUS_RE.exec(text)
  if (!match) return { message: text }
  return { status: Number(match[1]), message: match[2] }
}

/**
 * Return a typed error for a recognisable SDK error, else `err` unchanged. Anything without a
 * recognisable status is passed through -- transport failures and the SDK's own client-side errors
 * are not ours to reinterpret.
 */
export function translate(err: unknown): unknown {
  const { status, message } = splitStatus(err)
  if (status === undefined) return err
  const Cls = BY_STATUS[status] ?? SandboxGatewayError
  return new Cls(message, status, { cause: err })
}

/**
 * True for what the gateway marked retryable, false for anything a retry cannot fix.
 *
 * The whole rule is "did the gateway say 503" -- it separates retryable conditions from exhausted
 * quota at the status-code level, so nothing here reads a message. An earlier version matched a
 * bare `"timeout"` substring and so retried argument errors the caller had made, each costing a
 * cold-start window to fail.
 */
export function isRetryable(err: unknown): boolean {
  if (err instanceof SandboxThrottled) return true
  if (err instanceof Error) {
    if (err.name === 'AbortError' || err.name === 'TimeoutError') return true
    const code = (err.cause as { code?: string } | undefined)?.code
    if (code && ['ECONNRESET', 'ECONNREFUSED', 'ETIMEDOUT', 'UND_ERR_CONNECT_TIMEOUT'].includes(code)) {
      return true
    }
  }
  return false
}

// ==============================================================================================
// The patch
// ==============================================================================================

/**
 * A credential, or a synchronous accessor for one.
 *
 * Synchronous by design: `Sandbox.list()` returns a paginator without awaiting, so an async
 * resolver could not be honoured there without changing an official signature. To rotate a
 * short-lived token, have a refresh loop write it somewhere and pass `() => cache.token`.
 */
export type ApiKey = string | (() => string | undefined) | undefined

export interface InitOptions {
  /**
   * Gateway base URL, in any shape. Falls back to `E2B_API_URL` then `E2B_DOMAIN`.
   */
  url?: string
  /**
   * A Supabase JWT: a user session token, or a `service_role` key from a backend. Falls back to
   * `E2B_API_KEY`.
   */
  apiKey?: ApiKey
  /**
   * Must exceed the gateway's own cold-start ceiling (5 min), or the client gives up first and
   * reports a transport timeout instead of the gateway's actual error.
   */
  requestTimeoutMs?: number
  /**
   * Default arguments for every `Sandbox.create`, e.g.
   * `{ secure: true, timeoutMs: 900000, metadata: { ownerUserId: 'u_123' } }`. Anything passed at
   * the call site wins; `metadata` is merged rather than replaced. Defaults only -- this never
   * inspects or rejects what the caller asked for.
   */
  createDefaults?: Record<string, any>
  /**
   * Extra attempts when `create` fails with something the gateway marked retryable (503). Quota
   * and argument errors are never retried.
   */
  createRetries?: number
  createRetryBackoffMs?: number
  /**
   * Pass `Sandbox` from `@e2b/code-interpreter` to route its `runCode` channel through the gateway.
   * An optional peer dependency, so it is not imported by this module.
   */
  CodeInterpreterSandbox?: any
}

interface Undo {
  target: any
  name: string
  descriptor?: PropertyDescriptor
}

function readEnv(name: string): string | undefined {
  try {
    return typeof process !== 'undefined' && process.env ? process.env[name] : undefined
  } catch {
    return undefined
  }
}

let active: GatewayPatch | null = null

/** Handle for an installed patch. Returned by {@link initBytedSupabaseSandbox}. */
export class GatewayPatch {
  /** The normalised gateway base, ending in `/sandbox/v1`. */
  readonly apiUrl: string
  readonly createRetries: number
  readonly createRetryBackoffMs: number

  private readonly apiKeyOption: ApiKey
  private readonly requestTimeoutMs?: number
  private readonly createDefaults: Record<string, any>
  private readonly undo: Undo[] = []

  constructor(opts: InitOptions & { apiUrl: string }) {
    this.apiUrl = opts.apiUrl
    this.createRetries = opts.createRetries ?? 2
    this.createRetryBackoffMs = opts.createRetryBackoffMs ?? 3000
    this.apiKeyOption = opts.apiKey
    this.requestTimeoutMs = opts.requestTimeoutMs
    this.createDefaults = opts.createDefaults ?? {}
  }

  /** Resolve the credential, calling the supplied accessor if there is one. */
  resolveApiKey(): string | undefined {
    return typeof this.apiKeyOption === 'function' ? this.apiKeyOption() : this.apiKeyOption
  }

  /**
   * Official `ConnectionOpts` for this gateway. Also useful directly, for the few static entry
   * points not wrapped here, e.g. `Sandbox.listSnapshots(patch.connectionOptions())`.
   */
  connectionOptions(): Record<string, any> {
    const opts: Record<string, any> = {
      apiUrl: this.apiUrl,
      // The control plane and envd share one prefix; envd requests are recognised by the
      // E2b-Sandbox-Id header the SDK always sends.
      sandboxUrl: this.apiUrl,
      // The credential is a Supabase JWT, which does not match e2b's `e2b_<hex>` format.
      validateApiKey: false,
    }
    const key = this.resolveApiKey()
    if (key) opts.apiKey = key
    if (this.requestTimeoutMs !== undefined) opts.requestTimeoutMs = this.requestTimeoutMs
    return opts
  }

  /** Merge connection options into caller-supplied opts, letting the caller win. */
  withOptions(opts?: Record<string, any>): Record<string, any> {
    return { ...this.connectionOptions(), ...(opts ?? {}) }
  }

  /**
   * Supply default `create` arguments. Anything the caller passed always wins, and `metadata` is
   * merged rather than replaced so a per-call entry does not drop the defaults.
   *
   * Defaults only -- this fills in arguments, it never inspects or rejects them.
   */
  applyCreateDefaults(opts?: Record<string, any>): Record<string, any> {
    const merged = { ...this.createDefaults, ...(opts ?? {}) }
    if (this.createDefaults.metadata || opts?.metadata) {
      merged.metadata = { ...(this.createDefaults.metadata ?? {}), ...(opts?.metadata ?? {}) }
    }
    return merged
  }

  /**
   * Public URL for a port inside the sandbox.
   *
   * The trailing slash is load-bearing: without it a served page's relative sub-resources resolve
   * against the parent path and drop the `{port}` segment. The gateway 301s to add it.
   */
  previewUrl(sandboxId: string, port: number): string {
    return `${this.apiUrl}/proxy/${encodeURIComponent(sandboxId)}/${port}/`
  }

  /**
   * URL for the path-addressed envd files channel.
   *
   * Signed URLs get opened by a browser or fetch, neither of which can carry the `E2b-Sandbox-Id`
   * header -- hence the id in the path. The signature itself is still computed by the official SDK:
   * `downloadUrl`/`uploadUrl` call `fileUrl` and then append the signature params.
   */
  envdFilesUrl(sandboxId: string, path?: string, username?: string): string {
    const url = new URL(`${this.apiUrl}/envd/${encodeURIComponent(sandboxId)}/files`)
    if (username) url.searchParams.set('username', username)
    if (path) url.searchParams.set('path', path)
    return url.toString()
  }

  /**
   * URL for the run_code channel.
   *
   * Deliberately the dedicated `/jupyter/` channel rather than `/proxy/{id}/49999/`: it needs no
   * traffic-access token, and it rewrites the "no kernel listening" upstream 502 into a 404 whose
   * body actually reaches the caller (the code-interpreter SDK turns any 502 into a misleading
   * `TimeoutError`).
   */
  jupyterUrl(sandboxId: string): string {
    return `${this.apiUrl}/jupyter/${encodeURIComponent(sandboxId)}`
  }

  async retrying<T>(call: () => Promise<T>): Promise<T> {
    const attempts = Math.max(0, this.createRetries) + 1
    const backoff = Math.max(0, this.createRetryBackoffMs)
    let lastErr: unknown
    for (let attempt = 1; attempt <= attempts; attempt++) {
      try {
        return await call()
      } catch (raw) {
        const err = translate(raw)
        lastErr = err
        if (attempt >= attempts || !isRetryable(err)) throw err
        if (backoff) await new Promise((resolve) => setTimeout(resolve, backoff))
      }
    }
    throw lastErr
  }

  async typed<T>(call: () => Promise<T>): Promise<T> {
    try {
      return await call()
    } catch (raw) {
      throw translate(raw)
    }
  }

  /** Replace `target[name]`, remembering how to put it back. */
  set(target: any, name: string, value: unknown): void {
    this.undo.push({ target, name, descriptor: Object.getOwnPropertyDescriptor(target, name) })
    Object.defineProperty(target, name, { configurable: true, writable: true, value })
  }

  define(target: any, name: string, descriptor: PropertyDescriptor): void {
    this.undo.push({ target, name, descriptor: Object.getOwnPropertyDescriptor(target, name) })
    Object.defineProperty(target, name, { configurable: true, ...descriptor })
  }

  /** Undo the patch, returning the official classes to their original behaviour. */
  restore(): void {
    for (const { target, name, descriptor } of [...this.undo].reverse()) {
      if (descriptor) Object.defineProperty(target, name, descriptor)
      else delete target[name]
    }
    this.undo.length = 0
    if (active === this) active = null
  }
}

function patchUrls(patch: GatewayPatch, SandboxClass: any): void {
  patch.set(SandboxClass.prototype, 'getHost', function getHost(this: any, port: number) {
    return patch.previewUrl(this.sandboxId, port)
  })
  /**
   * Full public URL for `port`. Alias of `getHost` with an unambiguous name -- it returns a
   * complete URL including scheme, unlike upstream e2b where `getHost` returns a bare host meant to
   * be prefixed with `https://`. A gateway address has a path component, which a bare host cannot
   * express.
   */
  patch.set(SandboxClass.prototype, 'getPreviewUrl', function getPreviewUrl(this: any, port: number) {
    return patch.previewUrl(this.sandboxId, port)
  })
  // `fileUrl` is TypeScript-private but a plain prototype method at runtime, and both downloadUrl
  // and uploadUrl route through it -- so patching it here keeps every bit of the official signature
  // logic and only changes URL assembly.
  patch.set(SandboxClass.prototype, 'fileUrl', function fileUrl(this: any, path?: string, username?: string) {
    return patch.envdFilesUrl(this.sandboxId, path, username)
  })
}

function patchControlPlane(patch: GatewayPatch, SandboxClass: any): void {
  // Statics live on the base class (SandboxApi), so read through the prototype chain but shadow on
  // SandboxClass itself; restore then simply deletes the shadow.
  const original: Record<string, any> = {}
  for (const name of ['create', 'connect', 'list', 'kill', 'getInfo', 'getMetrics', 'setTimeout']) {
    original[name] = SandboxClass[name]
  }

  patch.set(SandboxClass, 'create', function create(
    this: any,
    templateOrOpts?: string | Record<string, any>,
    maybeOpts?: Record<string, any>
  ) {
    const template = typeof templateOrOpts === 'string' ? templateOrOpts : undefined
    const raw = typeof templateOrOpts === 'string' ? maybeOpts : templateOrOpts
    const opts = patch.withOptions(patch.applyCreateDefaults(raw))
    return patch.retrying(() =>
      template === undefined
        ? original.create.call(this, opts)
        : original.create.call(this, template, opts)
    )
  })

  patch.set(SandboxClass, 'connect', function connect(
    this: any,
    sandboxId: string,
    opts?: Record<string, any>
  ) {
    return patch.typed(() => original.connect.call(this, sandboxId, patch.withOptions(opts)))
  })

  patch.set(SandboxClass, 'list', function list(this: any, opts?: Record<string, any>) {
    // Sync: it builds a paginator, it does not perform the request.
    return original.list.call(this, patch.withOptions(opts))
  })

  for (const name of ['kill', 'getInfo', 'getMetrics']) {
    patch.set(SandboxClass, name, function wrapped(
      this: any,
      sandboxId: string,
      opts?: Record<string, any>
    ) {
      return patch.typed(() => original[name].call(this, sandboxId, patch.withOptions(opts)))
    })
  }

  patch.set(SandboxClass, 'setTimeout', function setTimeoutWrapped(
    this: any,
    sandboxId: string,
    timeoutMs: number,
    opts?: Record<string, any>
  ) {
    return patch.typed(() =>
      original.setTimeout.call(this, sandboxId, timeoutMs, patch.withOptions(opts))
    )
  })
}

/**
 * Point the official E2B SDK at a Volcengine Supabase compute-gateway instance.
 *
 * Call once at startup, then use `e2b` exactly as its own documentation describes. Installing again
 * replaces the previous installation -- this is process-global state, so one process serves one
 * gateway and one credential at a time.
 *
 * @returns a handle whose `restore()` undoes the patch.
 */
export function initBytedSupabaseSandbox(options: InitOptions = {}): GatewayPatch {
  const resolved = options.url || readEnv('E2B_API_URL') || readEnv('E2B_DOMAIN')
  const patch = new GatewayPatch({
    ...options,
    apiUrl: normalizeGatewayUrl(resolved),
    apiKey: options.apiKey ?? readEnv('E2B_API_KEY'),
  })

  // Never stack patches: a second install would capture the first one's wrappers as its
  // "originals" and a single restore would only peel off one layer.
  if (active) active.restore()

  patchUrls(patch, Sandbox)
  patchControlPlane(patch, Sandbox)

  // The code-interpreter class extends e2b's Sandbox, so it inherits everything patched above; the
  // one thing it defines itself is the jupyterUrl getter.
  const CodeInterpreter = options.CodeInterpreterSandbox
  if (CodeInterpreter) {
    patch.define(CodeInterpreter.prototype, 'jupyterUrl', {
      get(this: any) {
        return patch.jupyterUrl(this.sandboxId)
      },
    })
  }

  active = patch
  return patch
}

/** The currently installed patch, if any. */
export function activePatch(): GatewayPatch | null {
  return active
}
