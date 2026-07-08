# Byted Supabase CLI

This is the Byted / Volcengine distribution of the [Supabase CLI](https://github.com/supabase/cli), published to npm as [`@byted-supabase/cli`](https://www.npmjs.com/package/@byted-supabase/cli).

- [x] Running Supabase locally
- [x] Managing database migrations
- [x] Creating and deploying Supabase Functions
- [x] Generating types directly from your database schema
- [x] Making authenticated HTTP requests to [Management API](https://supabase.com/docs/reference/api/introduction)

## Getting started

### Install the CLI

Available via [npm](https://www.npmjs.com/package/@byted-supabase/cli). The guided setup installs the CLI globally and installs the matching `byted-supabase` agent skill:

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

### Run the CLI

```bash
byted-supabase-cli bootstrap
```

Or via npx, without installing:

```bash
npx @byted-supabase/cli bootstrap
```

### Update

```bash
byted-supabase-cli update
```

## Docs

Command & config reference can be found [here](https://supabase.com/docs/reference/cli/about).

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

This project does not accept external contributions or pull requests.
