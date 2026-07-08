# Makefile for @byted-supabase/cli
#
# Publish to public npmjs (version comes from the git tag, never typed by hand):
#   git tag v0.1.3 && git push origin v0.1.3   # the tag is the single source of truth
#   make npm-release                           # build all platforms + publish 7 packages
#
# Distribution model: the npm package ships NO binary itself. The native binary
# lives in one of six per-platform optional dependencies
# (@byted-supabase/cli-<os>-<arch>); npm installs only the one matching the host,
# so there is no install-time CDN download. scripts/release.mjs orchestrates the
# whole release (safety rails: master-only, clean tree, tag-derived version, and
# never overwriting an already-published version) and publishes the six platform
# packages first, the main package last. CGO is disabled, so one build host
# cross-compiles every target — no goreleaser, no per-OS runners.
#
# Run `make npm-release-dry` first to rehearse (builds + `npm publish --dry-run`,
# no upload). Auth: set NODE_AUTH_TOKEN (npm Automation token) or `npm login`.

# Nearest git tag, with the leading "v" stripped (v0.1.3 -> 0.1.3). Used only by
# build-cli; the release script reads the tag itself.
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')

DIST    ?= dist
BINARY  ?= byted-supabase-cli

.DEFAULT_GOAL := help

.PHONY: help npm-release npm-release-dry build-cli

# Self-documenting help: lists every target with a `## comment` on its rule line.
help:
	@echo "Usage: make <target>"
	@echo
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort \
	  | awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# Build the CLI for the host platform into $(DIST)/. Version is injected the same
# way goreleaser stamps it (-X internal/utils.Version), so `byted-supabase-cli
# --version` reports the git tag instead of an empty string.
build-cli: ## Build the CLI into ./dist for the host platform
	CGO_ENABLED=0 go build -trimpath \
	  -ldflags "-s -w -X github.com/volcengine/byted-supabase-cli/internal/utils.Version=$(VERSION)" \
	  -o $(DIST)/$(BINARY) main.go
	@echo "built $(DIST)/$(BINARY) (version: $(if $(VERSION),$(VERSION),dev))"

# Cross-compile every platform and publish all 7 packages. All safety rails live
# in scripts/release.mjs (see header). Requires npm auth (NODE_AUTH_TOKEN / login).
npm-release: ## Build all platforms and publish to npmjs (version from git tag)
	node scripts/release.mjs

# Same as npm-release but `npm publish --dry-run`: builds + packs every package,
# enforces the rails, uploads nothing. Use this to rehearse a release.
npm-release-dry: ## Rehearse a release (build + pack, no upload)
	node scripts/release.mjs --dry-run
