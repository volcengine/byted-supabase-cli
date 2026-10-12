import os
import json
from urllib.error import HTTPError
from urllib.request import Request, urlopen

from byted_supabase_sandbox import init_byted_supabase_sandbox
from e2b import Sandbox


def required(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"missing environment variable: {name}")
    return value


def optional(name: str) -> str | None:
    return os.environ.get(name, "").strip() or None


def auth_request(url: str, path: str, body: dict) -> tuple[int, dict]:
    request = Request(
        f"{url}/auth/v1/{path}",
        data=json.dumps(body).encode(),
        headers={
            "apikey": required("SUPABASE_ANON_KEY"),
            "content-type": "application/json",
        },
        method="POST",
    )
    try:
        with urlopen(request) as response:
            return response.status, json.load(response)
    except HTTPError as error:
        try:
            return error.code, json.load(error)
        except json.JSONDecodeError:
            return error.code, {}


def resolve_access_token(url: str) -> str:
    access_token = optional("SUPABASE_ACCESS_TOKEN")
    if access_token:
        return access_token

    email = required("SUPABASE_EMAIL")
    password = required("SUPABASE_PASSWORD")
    signup_status, signup_body = auth_request(
        url, "signup", {"email": email, "password": password}
    )
    if signup_status >= 300 and signup_status != 422:
        detail = signup_body.get("msg") or signup_body.get("error_description") or "unknown error"
        raise RuntimeError(f"Supabase Auth signup failed ({signup_status}): {detail}")

    login_status, login_body = auth_request(
        url,
        "token?grant_type=password",
        {"email": email, "password": password},
    )
    access_token = login_body.get("access_token")
    if login_status >= 300 or not access_token:
        detail = login_body.get("msg") or login_body.get("error_description") or "access_token was not returned"
        raise RuntimeError(
            f"Supabase Auth login failed ({login_status}): {detail}. "
            "If email confirmation is enabled, confirm the test account before running again."
        )
    return access_token


def main() -> None:
    supabase_url = required("SUPABASE_URL")
    init_byted_supabase_sandbox(
        url=supabase_url,
        api_key=resolve_access_token(supabase_url),
    )

    with Sandbox.create(
        "base",
        timeout=300,
    ) as sandbox:
        result = sandbox.commands.run("python3 -c 'print(\"hello world\")'")
        stdout = getattr(result, "stdout", "") or ""
        stderr = getattr(result, "stderr", "") or ""
        exit_code = getattr(result, "exit_code", None)

        if stdout:
            print(stdout, end="")
        if stderr:
            print(stderr, end="", file=os.sys.stderr)
        if exit_code is not None and exit_code != 0:
            raise RuntimeError(f"sandbox command failed with exit_code={exit_code}")
        if "hello world" not in stdout:
            raise RuntimeError("sandbox verification failed: expected output was not found")
        print("sandbox verification passed")


if __name__ == "__main__":
    main()
