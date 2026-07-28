"""Point the official E2B SDK at a Volcengine Supabase Sandbox instance.

    from byted_supabase_sandbox import init_byted_supabase_sandbox

    init_byted_supabase_sandbox(url="https://<your-instance>", api_key="<supabase jwt>")

    from e2b import Sandbox
    sbx = Sandbox.create()

Everything after that call is stock E2B usage -- same imports, same API, same documentation.
"""

from .e2b_gateway import *  # noqa: F401,F403
from .e2b_gateway import __all__  # noqa: F401
