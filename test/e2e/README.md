# Live e2e tests

Black-box end-to-end tests that drive the **compiled CLI binary** against a
**real Volcengine account**: `TestMain` provisions one Supabase workspace per
run and the tests exercise CLI commands against it. **The harness never
deletes anything by default** — it prints the exact cleanup commands and
leaves the workspace for you to remove by hand; automatic teardown is opt-in
via `BYTED_SUPABASE_E2E_DELETE=1` (for CI). Guarded by the `live_e2e` build
tag so `go test ./...` never touches the cloud. (The shell scripts under
`tests/` are unrelated — they smoke the local `start` docker stack.)

## Running

```sh
make build-cli
export VOLCENGINE_ACCESS_KEY=...   # test account with the AIDAP service-linked role enabled
export VOLCENGINE_SECRET_KEY=...
export VOLCENGINE_REGION=cn-beijing
make e2e-live                      # = go test -tags live_e2e -v -timeout 45m ./test/e2e/...
```

Creating a workspace bills the account. Keep runs on a dedicated test account,
and consider `BYTED_SUPABASE_E2E_CREATE_ARGS="--suspend-timeout-seconds 300"`.

## Environment contract

| Variable | Meaning |
|---|---|
| `VOLCENGINE_ACCESS_KEY` / `VOLCENGINE_SECRET_KEY` | Required (headless auth). Without them the harness refuses to provision — set `BYTED_SUPABASE_E2E_USE_PROFILE=1` to explicitly opt into the local `~/.volcengine/config.json` profile instead. |
| `VOLCENGINE_REGION` | Target region; the CLI defaults to `cn-beijing` when unset. |
| `BYTED_SUPABASE_E2E_BIN` | Path to the binary under test. Default: `dist/byted-supabase-cli` (from `make build-cli`). |
| `BYTED_SUPABASE_E2E_WORKSPACE` | Reuse this existing workspace instead of provisioning; the harness never deletes a reused workspace. For local debugging. |
| `BYTED_SUPABASE_E2E_DELETE` | Opt into automatic teardown: delete the provisioned workspace when the run ends (deletion protection is lifted first). Meant for CI; unset, cleanup is manual. |
| `BYTED_SUPABASE_E2E_CREATE_ARGS` | Extra whitespace-separated args for `projects create` (e.g. `--is-agent-plan --suspend-timeout-seconds 300`). |
| `BYTED_SUPABASE_E2E_READY_TIMEOUT` | How long to wait for `Running` (Go duration, default `15m`). |
| `BYTED_SUPABASE_E2E_DATA_PLANE` | Opt into the data-plane suite (secrets/functions/storage/db query/gen types). It **enables public endpoint access** on the test workspace's default branch (disabled again afterwards), hence off by default. |

## Test tiers

| File | Coverage | Mutates? |
|---|---|---|
| `smoke_test.go` | workspace detail, api-keys, operations | no |
| `readonly_test.go` | projects list/overview, endpoints list, branches list, `--version`, `update --check` | no |
| `manage_test.go` | rename cycle, tags create/delete | reversible, restores state |
| `link_test.go` | init → link → unlink flow (skipped without env AK/SK) | local files only |
| `dataplane_test.go` | secrets/functions/storage/db query/gen types via the branch gateway | opt-in; toggles public endpoint access |

## Lifecycle & cleanup

Workspaces are named `cli-e2e-live-<run-id>-<rand>` (run id from
`GITHUB_RUN_ID` / `CI_PIPELINE_ID` / `BUILD_ID`, else a timestamp) so they are
easy to spot and a CI sweep can target exactly one run's leftovers.

By default every run ends by printing the two cleanup commands (deletion
protection is on by default, hence two steps):

```sh
byted-supabase-cli projects deletion-protection <id> --disable --yes
byted-supabase-cli projects delete <id> --yes
```

With `BYTED_SUPABASE_E2E_DELETE=1` (CI), the harness runs these itself — also
for a half-provisioned workspace when setup fails — and a failed deletion
fails the run loudly. Sweep orphans by filtering `projects list --output json`
for `name =~ ^cli-e2e-live-`.

## Known constraints

- The test account must have the AIDAP service-linked role enabled, or
  `projects create` fails before provisioning.
- New workspaces have **deletion protection enabled by default**
  (`deletion_protection: "Enabled"`), so teardown always runs
  `deletion-protection --disable --yes` before `delete` — keep that in mind
  when sweeping orphans manually too.
- `projects api-keys` names keys `AnonKey` / `ServiceRoleKey`; tests match on
  the stable `type` field (`Public` / `Service`) instead.
- Data-plane commands (storage/functions/db) need a reachable workspace
  endpoint. Some CI environments may need an endpoint fallback mechanism in the
  branch under test. The current smoke tests stick to control-plane commands and
  do not require it.
