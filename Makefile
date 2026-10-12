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

# Full git describe, with the leading "v" stripped. On a tagged commit this is
# the clean version (v0.1.3 -> 0.1.3); between tags it carries the distance and
# commit (0.1.3-8-gc45ca44), so a local dev build is distinguishable from the
# published release at a glance. Used only by build-cli; the release script
# reads the tag itself.
VERSION ?= $(shell git describe --tags 2>/dev/null | sed 's/^v//')

DIST    ?= dist
BINARY  ?= byted-supabase-cli

# Skill 打包配置
SKILLS_DIR ?= skills
ZIP_DIR ?= .tmp
SKILL_ZIP_NAMES ?= byted-supabase
ZIP_EXCLUDES := '*/.DS_Store' '*__pycache__*' '*.pyc' '*/.venv/*' '*/.ruff_cache/*' '*/output/*'

# AgentBuddy skill 发布配置
SKILL_NAME ?= byted-supabase
NAMESPACE ?=
BYTEDCLI ?= bytedcli
SKILL_KEEP := SKILL.md references scripts assets

# AgentKit 分发仓库同步配置。源目录是当前仓库的 skills/byted-supabase，
# DIST 保留给本地构建产物和 zip，避免影响 make build / make skill-zip。
SKILL_DIST ?= ../volcengine-agentkit-samples
SKILL_DIST_RSYNC_OPTS := -a --delete \
	--exclude='.DS_Store' \
	--exclude='__pycache__/' \
	--exclude='*.pyc'

.DEFAULT_GOAL := help

.PHONY: help npm-release npm-release-dry build-cli e2e-live skill-sync skill-sync-dist skill-sync-dist-check zip skill-zip

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
build: ## Build the CLI into ./dist for the host platform
	CGO_ENABLED=0 go build -trimpath \
	  -ldflags "-s -w -X github.com/volcengine/byted-supabase-cli/internal/utils.Version=$(VERSION)" \
	  -o $(DIST)/$(BINARY) main.go
	@echo "built $(DIST)/$(BINARY) (version: $(if $(VERSION),$(VERSION),dev))"

# Live e2e: drives $(DIST)/$(BINARY) against a real Volcengine account —
# creates and deletes a workspace, so it bills the account. Auth and knobs are
# documented in test/e2e/README.md. Kept out of `go test ./...` by the
# live_e2e build tag.
e2e-live: build-cli ## Run live e2e tests against a real Volcengine account (creates a workspace)
	go test -tags live_e2e -v -timeout 45m -count=1 ./test/e2e/...

# Cross-compile every platform and publish all 7 packages. All safety rails live
# in scripts/release.mjs (see header). Requires npm auth (NODE_AUTH_TOKEN / login).
npm-release: ## Build all platforms and publish to npmjs (version from git tag)
	node scripts/release.mjs

# Same as npm-release but `npm publish --dry-run`: builds + packs every package,
# enforces the rails, uploads nothing. Use this to rehearse a release.
npm-release-dry: ## Rehearse a release (build + pack, no upload)
	node scripts/release.mjs --dry-run

# Publish the checked-in skill to AgentBuddy. Only the runtime skill assets are
# staged; repository metadata and channel-specific files are intentionally omitted.
skill-sync: ## Publish the skill to AgentBuddy
	@command -v $(firstword $(BYTEDCLI)) >/dev/null 2>&1 || { echo "❌ 未找到 bytedcli（可通过 BYTEDCLI= 覆盖）"; exit 1; }
	@test -n "$(strip $(NAMESPACE))" || { echo "❌ 需指定命名空间：make skill-sync NAMESPACE=<owner>/<name>"; exit 1; }
	@test -d "$(SKILLS_DIR)/$(SKILL_NAME)" || { echo "❌ skill 不存在: $(SKILLS_DIR)/$(SKILL_NAME)"; exit 1; }
	@test -f "$(SKILLS_DIR)/$(SKILL_NAME)/SKILL.md" || { echo "❌ skill 缺少 SKILL.md: $(SKILLS_DIR)/$(SKILL_NAME)"; exit 1; }
	@stage="$$(mktemp -d)"; \
		trap 'rm -rf "$$stage"' EXIT; \
		mkdir -p "$$stage/$(SKILL_NAME)"; \
		for keep in $(SKILL_KEEP); do \
			if [ -e "$(SKILLS_DIR)/$(SKILL_NAME)/$$keep" ]; then \
				cp -R "$(SKILLS_DIR)/$(SKILL_NAME)/$$keep" "$$stage/$(SKILL_NAME)/"; \
			fi; \
		done; \
		echo "→ 发布 $(SKILL_NAME) → AgentBuddy namespace=$(NAMESPACE)"; \
		$(BYTEDCLI) agentbuddy skill publish \
			--path "$$stage/$(SKILL_NAME)" \
			--namespace "$(NAMESPACE)" \
			--name "$(SKILL_NAME)"
	@echo "✅ skill 已同步到 AgentBuddy"

skill-sync-dist: ## 同步本仓库 Skill 到 AgentKit 分发仓库
	@test -d "$(SKILL_DIST)" || { echo "❌ AgentKit 分发仓库不存在: $(SKILL_DIST)"; exit 1; }
	@test -d "$(SKILL_DIST)/skills" || { echo "❌ 分发仓库缺少 skills/ 目录: $(SKILL_DIST)"; exit 1; }
	@test -d "$(SKILLS_DIR)/$(SKILL_NAME)" || { echo "❌ 源 skill 不存在: $(SKILLS_DIR)/$(SKILL_NAME)"; exit 1; }
	@test -f "$(SKILLS_DIR)/$(SKILL_NAME)/SKILL.md" || { echo "❌ 源 skill 缺少 SKILL.md: $(SKILLS_DIR)/$(SKILL_NAME)/SKILL.md"; exit 1; }
	@mkdir -p "$(SKILL_DIST)/skills/$(SKILL_NAME)"
	@echo "→ 同步 $(SKILLS_DIR)/$(SKILL_NAME) → $(SKILL_DIST)/skills/$(SKILL_NAME)"
	@rsync $(SKILL_DIST_RSYNC_OPTS) \
		"$(SKILLS_DIR)/$(SKILL_NAME)/" \
		"$(SKILL_DIST)/skills/$(SKILL_NAME)/"
	@echo "✅ Skill 已同步到 AgentKit 分发仓"

skill-sync-dist-check: ## 预览同步 Skill 到 AgentKit 分发仓的差异
	@test -d "$(SKILL_DIST)" || { echo "❌ AgentKit 分发仓库不存在: $(SKILL_DIST)"; exit 1; }
	@test -d "$(SKILL_DIST)/skills" || { echo "❌ 分发仓库缺少 skills/ 目录: $(SKILL_DIST)"; exit 1; }
	@test -d "$(SKILLS_DIR)/$(SKILL_NAME)" || { echo "❌ 源 skill 不存在: $(SKILLS_DIR)/$(SKILL_NAME)"; exit 1; }
	@echo "=== dry-run: $(SKILLS_DIR)/$(SKILL_NAME) → $(SKILL_DIST)/skills/$(SKILL_NAME) ==="
	@rsync $(SKILL_DIST_RSYNC_OPTS) -n -v \
		"$(SKILLS_DIR)/$(SKILL_NAME)/" \
		"$(SKILL_DIST)/skills/$(SKILL_NAME)/" \
		| grep -vE '^(sending|sent |total size|$$)' || true
	@echo "（以上为将发生的改动；无输出表示已一致）"

# Package the checked-in skills as standalone zip archives. Each archive keeps
# the skill directory as its top-level directory so it can be extracted into
# a skills directory without losing the skill name.
zip: ## Package skills as zip archives in ./.tmp
	@command -v zip >/dev/null 2>&1 || { echo "❌ 未找到 zip 命令"; exit 1; }
	@test -n "$(strip $(SKILL_ZIP_NAMES))" || { echo "❌ 没有可打包的 skill"; exit 1; }
	@mkdir -p "$(ZIP_DIR)"
	@set -eu; \
	for skill in $(SKILL_ZIP_NAMES); do \
		dir="$(SKILLS_DIR)/$$skill"; \
		test -d "$$dir" || { echo "❌ skill 不存在: $$dir"; exit 1; }; \
		test -f "$$dir/SKILL.md" || { echo "❌ skill 缺少 SKILL.md: $$dir/SKILL.md"; exit 1; }; \
		archive="$(abspath $(ZIP_DIR))/$$skill.zip"; \
		rm -f "$$archive"; \
		(cd "$(SKILLS_DIR)" && zip -q -r -X "$$(cd -P "$$(dirname "$$archive")" && pwd)/$$(basename "$$archive")" "$$skill" -x $(ZIP_EXCLUDES)); \
		echo "✅ 已生成 $$archive"; \
	done

skill-zip: zip ## Compatibility alias for zip
