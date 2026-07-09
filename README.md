# Byted Supabase CLI

[![npm version](https://img.shields.io/npm/v/%40byted-supabase%2Fcli.svg)](https://www.npmjs.com/package/@byted-supabase/cli)
[![license](https://img.shields.io/npm/l/%40byted-supabase%2Fcli.svg)](LICENSE)
![platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

`byted-supabase-cli` is the command-line tool for the Volcengine AI-native BaaS platform (Supabase edition). It lets you manage workspaces, run SQL, deploy Edge Functions, move storage objects, host static frontends, and generate TypeScript types — all from one terminal command.

## Why this CLI?

This is the Byted / Volcengine distribution of the official [Supabase CLI](https://github.com/supabase/cli), published to npm as [`@byted-supabase/cli`](https://www.npmjs.com/package/@byted-supabase/cli). It keeps the command shapes you know from upstream where they apply, and targets the Volcengine-operated platform instead of supabase.com:

- Authenticates with your **Volcengine account** instead of supabase.com.
- Manages **Volcengine Supabase workspaces**, preview branches, computes, and endpoints.
- Adds platform capabilities that upstream does not have, such as static frontend hosting and first-class support for AI coding agents.

If your backend runs on Volcengine Supabase, this is the CLI to use.

## Features

- **Workspace management** — create, list, start/stop, and configure workspaces (`projects`), preview branches (`branches`), computes (`computes`), endpoints (`endpoints`), and network restrictions (`network-restrictions`).
- **Database access** — run SQL against a linked project with `db query`, pull or dump remote schemas (`db pull`, `db dump`), get connection strings, and check performance with `db advisors` and `inspect db`.
- **Edge Functions** — scaffold (`functions new`), deploy, download, and list Deno-based Edge Functions, with `secrets` for their configuration.
- **Storage** — manage buckets and objects with `storage ls / cp / mv / rm`, and provision buckets declaratively with `seed buckets`.
- **Auth configuration** — manage auth settings, hooks, and third-party providers with `auth`.
- **Type generation** — `gen types` produces TypeScript (or Go / Swift) types straight from your Postgres schema.
- **Frontend hosting** — deploy static frontends alongside your workspace with `pages`.
- **Built for AI agents** — `mcp serve` starts a Model Context Protocol server over stdio, and `skills install` installs the matching `byted-supabase` agent skill, so AI assistants like Claude Code can drive the platform for you.

## Getting started

### Install

The guided setup installs the CLI globally and the matching `byted-supabase` agent skill:

```bash
npx @byted-supabase/cli@latest install
```

To install only the CLI and get the `byted-supabase-cli` command:

```bash
npm install -g @byted-supabase/cli
```

Or add it as a project dev dependency:

```bash
npm i @byted-supabase/cli --save-dev
```

> npm installs the matching platform package for macOS / Linux / Windows on x86_64 / arm64. There is no postinstall download.

### Authenticate

```bash
byted-supabase-cli login
```

This opens a browser for OAuth login with your Volcengine account. On a headless machine, use `byted-supabase-cli login --remote` to sign in from another device. In CI, skip `login` entirely and set `VOLCENGINE_ACCESS_KEY` / `VOLCENGINE_SECRET_KEY` (or use `configure set`) instead. See [docs/supabase/login.md](docs/supabase/login.md) for details.

### Create and link a workspace

```bash
# Create a workspace on Volcengine Supabase
byted-supabase-cli projects create my-app

# Scaffold the local supabase/ directory and link it to the workspace
byted-supabase-cli init
byted-supabase-cli link --project-ref <workspace-id>
```

### Build

```bash
# Run SQL against the linked workspace
byted-supabase-cli db query "select now()"

# Scaffold and deploy an Edge Function
byted-supabase-cli functions new hello
byted-supabase-cli functions deploy hello

# Generate TypeScript types from the database schema
byted-supabase-cli gen types --linked --lang typescript
```

Explore the full command surface anytime:

```bash
byted-supabase-cli --help
```

### Update

```bash
byted-supabase-cli update
```

## Documentation

- Command reference: run `byted-supabase-cli --help` (or `<command> --help`) for the authoritative list — this distribution exposes a different command surface than the upstream Supabase CLI
- Extended man pages for each command: [docs/supabase](docs/supabase)
- Login modes and non-interactive behavior: [docs/supabase/login.md](docs/supabase/login.md)
- Library usage examples for building your own tooling: [examples](examples)

## Breaking changes

We follow semantic versioning for changes that directly impact CLI commands, flags, and configurations.

However, due to dependencies on other service images, we cannot guarantee that schema migrations, seed.sql, and generated types will always work for the same CLI major version. If you need such guarantees, we encourage you to pin a specific version of the CLI in package.json.

## Developing

To run from source:

```sh
# Go >= 1.25
go run . help
```

## Contributing

This project does not accept external contributions or pull requests. Bug reports are welcome on the [issue tracker](https://github.com/volcengine/byted-supabase-cli/issues).

## License

[MIT](LICENSE)
