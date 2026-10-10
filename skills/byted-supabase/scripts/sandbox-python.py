"""Complete Python SDK example: init -> create -> write -> execute -> verify -> kill.

Required environment variables:
  SUPABASE_URL
  SUPABASE_SERVICE_ROLE_KEY
  SANDBOX_OWNER_USER_ID

Run this file only in a backend or trusted worker. Never expose the
service-role key to a client or commit real credentials.
"""

import os

from byted_supabase_sandbox import init_byted_supabase_sandbox
from e2b import Sandbox


def required(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise RuntimeError(f"missing environment variable: {name}")
    return value


def main() -> None:
    init_byted_supabase_sandbox(
        url=required("SUPABASE_URL"),
        api_key=required("SUPABASE_SERVICE_ROLE_KEY"),
    )

    print("creating sandbox")
    with Sandbox.create(
        "base",
        timeout=600,
        metadata={"ownerUserId": required("SANDBOX_OWNER_USER_ID")},
    ) as sandbox:
        sandbox.files.write(
            "/tmp/hello.py",
            'print("hello from Supabase Sandbox")\n',
        )
        result = sandbox.commands.run("python3 /tmp/hello.py")
        stdout = getattr(result, "stdout", "") or ""
        stderr = getattr(result, "stderr", "") or ""
        exit_code = getattr(result, "exit_code", None)

        if stdout:
            print(stdout, end="")
        if stderr:
            print(stderr, end="", file=os.sys.stderr)
        if exit_code is not None and exit_code != 0:
            raise RuntimeError(f"sandbox command failed with exit_code={exit_code}")
        if "hello from Supabase Sandbox" not in stdout:
            raise RuntimeError("sandbox verification failed: expected output was not found")
        print("sandbox verification passed")


if __name__ == "__main__":
    main()
