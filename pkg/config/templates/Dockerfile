# Exposed for updates by .github/dependabot.yml
FROM supabase/postgres:17.11.0.001 AS pg
# Append to ServiceImages when adding new dependencies below
FROM library/kong:2.8.1 AS kong
FROM axllent/mailpit:v1.22.3 AS mailpit
FROM postgrest/postgrest:v14.10 AS postgrest
FROM supabase/postgres-meta:v0.99.0 AS pgmeta
FROM supabase/studio:2026.09.28-sha-5e59b60 AS studio
FROM darthsim/imgproxy:v3.8.0 AS imgproxy
FROM supabase/edge-runtime:v1.77.1 AS edgeruntime
FROM timberio/vector:0.53.0-alpine AS vector
FROM supabase/supavisor:2.9.13 AS supavisor
FROM supabase/gotrue:v2.197.0 AS gotrue
FROM supabase/realtime:v2.139.0 AS realtime
FROM supabase/storage-api:v1.79.23 AS storage
FROM supabase/logflare:1.50.15 AS logflare
# Append to JobImages when adding new dependencies below
FROM supabase/pgadmin-schema-diff:cli-0.0.5 AS differ
FROM supabase/migra:3.0.1663481299 AS migra
FROM supabase/pg_prove:3.36 AS pgprove
