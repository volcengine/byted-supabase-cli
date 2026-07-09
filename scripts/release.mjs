#!/usr/bin/env node

// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Release orchestrator for @byted-supabase/cli (optionalDependencies model).
//
// Publishes 7 packages from a single build host (Go + Node + npm):
//   - 6 per-platform packages @byted-supabase/cli-<os>-<arch>, each containing
//     ONE native binary plus an os/cpu gate so npm installs only the matching one;
//   - the main package @byted-supabase/cli, whose optionalDependencies pin all 6
//     at the exact release version. Platform packages are published FIRST and the
//     main package LAST, so the main package's deps always already exist.
//
// Safety rails — ALL enforced before anything is published:
//   1. Must run on the release branch (default: master; override RELEASE_BRANCH).
//   2. Working tree must be clean.
//   3. HEAD must carry exactly one semver tag vX.Y.Z; that tag IS the version.
//   4. None of the 7 packages may already exist at that version on the registry.
//      Versions are append-only — we never overwrite, only add. A partial release
//      (some packages already at this version) aborts: bump the version and retry.
//
// Auth: the npm "Automation" token is read from a config file kept OUTSIDE the
// repo (so re-cloning never touches the secret). Default path:
//   $RELEASE_CONFIG  or  ~/.config/byted-supabase-cli/release.env
// Format: KEY=VALUE lines ('#' comments). Recognized keys: NODE_AUTH_TOKEN
// (required), and optionally NPM_REGISTRY / RELEASE_BRANCH. Real environment
// variables win; the file only fills gaps. `npm login` also works as a fallback.
// The token is never written to disk by this script — the generated .npmrc
// references ${NODE_AUTH_TOKEN} and npm expands it from the environment.
//
// Usage:
//   node scripts/release.mjs            # real release
//   node scripts/release.mjs --dry-run  # build + assemble + `npm publish --dry-run` (no upload, no auth, rails still enforced)
"use strict";

import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const DRY_RUN = process.argv.includes("--dry-run");

const SCOPE = "@byted-supabase";
const MAIN_PKG = `${SCOPE}/cli`;
const BINARY = "byted-supabase-cli";

const info = (m) => console.log(`• ${m}`);
const warn = (m) => console.warn(`⚠ ${m}`);
const fail = (m) => {
  console.error(`\n✖ ${m}\n`);
  process.exit(1);
};
const capture = (cmd, args, opts = {}) =>
  execFileSync(cmd, args, { encoding: "utf8", cwd: ROOT, ...opts }).trim();
const run = (cmd, args, opts = {}) =>
  execFileSync(cmd, args, { stdio: "inherit", cwd: ROOT, ...opts });

// ---- load secrets/config from an absolute-path file (kept outside the repo) --
const CONFIG_PATH =
  process.env.RELEASE_CONFIG ||
  path.join(os.homedir(), ".config", "byted-supabase-cli", "release.env");
function loadConfig() {
  if (!fs.existsSync(CONFIG_PATH)) return;
  // Warn if the secret file is group/world-readable.
  try {
    const mode = fs.statSync(CONFIG_PATH).mode & 0o777;
    if (mode & 0o077) warn(`${CONFIG_PATH} is mode ${mode.toString(8)} — run: chmod 600 ${CONFIG_PATH}`);
  } catch {
    /* ignore stat errors */
  }
  let raw;
  try {
    raw = fs.readFileSync(CONFIG_PATH, "utf8");
  } catch (e) {
    fail(`Cannot read release config ${CONFIG_PATH}: ${e.message}`);
  }
  for (const line of raw.split("\n")) {
    const s = line.trim();
    if (!s || s.startsWith("#")) continue;
    const eq = s.indexOf("=");
    if (eq === -1) continue;
    const key = s.slice(0, eq).trim();
    let val = s.slice(eq + 1).trim();
    if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
      val = val.slice(1, -1);
    }
    if (key && process.env[key] === undefined) process.env[key] = val; // env wins
  }
  info(`Loaded release config from ${CONFIG_PATH}`);
}
loadConfig();

const REGISTRY = process.env.NPM_REGISTRY || "https://registry.npmjs.org/";
const RELEASE_BRANCH = process.env.RELEASE_BRANCH || "master";

// GOOS/GOARCH (Go build) <-> process.platform/process.arch (npm os/cpu + names).
const TARGETS = [
  { goos: "darwin", goarch: "amd64", os: "darwin", cpu: "x64", exe: "" },
  { goos: "darwin", goarch: "arm64", os: "darwin", cpu: "arm64", exe: "" },
  { goos: "linux", goarch: "amd64", os: "linux", cpu: "x64", exe: "" },
  { goos: "linux", goarch: "arm64", os: "linux", cpu: "arm64", exe: "" },
  { goos: "windows", goarch: "amd64", os: "win32", cpu: "x64", exe: ".exe" },
  { goos: "windows", goarch: "arm64", os: "win32", cpu: "arm64", exe: ".exe" },
];
const platformPkgName = (t) => `${SCOPE}/cli-${t.os}-${t.cpu}`;

// ---- rail 1: branch ----------------------------------------------------------
const branch = capture("git", ["rev-parse", "--abbrev-ref", "HEAD"]);
if (branch !== RELEASE_BRANCH) {
  fail(`Releases must run on '${RELEASE_BRANCH}', but HEAD is on '${branch}'.`);
}

// ---- rail 2: clean tree ------------------------------------------------------
const dirty = capture("git", ["status", "--porcelain"]);
if (dirty) {
  fail(`Working tree is not clean — commit or stash first:\n${dirty}`);
}

// ---- rail 3: exactly one semver tag at HEAD ----------------------------------
const tags = capture("git", ["tag", "--points-at", "HEAD"])
  .split("\n")
  .filter((t) => /^v\d+\.\d+\.\d+$/.test(t));
if (tags.length === 0) {
  fail(
    `HEAD has no semver tag (vX.Y.Z). Tag the release commit first, e.g.:\n` +
      `    git tag v0.1.3 && git push origin v0.1.3`
  );
}
if (tags.length > 1) {
  fail(`HEAD has multiple semver tags: ${tags.join(", ")}. Leave exactly one.`);
}
const VERSION = tags[0].slice(1); // strip leading 'v'
info(`Releasing ${VERSION} (tag ${tags[0]}) from branch '${branch}'${DRY_RUN ? " [dry-run]" : ""}`);

// ---- rail 4: never overwrite an existing version -----------------------------
const ALL_PKGS = [MAIN_PKG, ...TARGETS.map(platformPkgName)];
const versionExists = (pkg) => {
  try {
    const out = execFileSync("npm", ["view", `${pkg}@${VERSION}`, "version", "--registry", REGISTRY], {
      encoding: "utf8",
      cwd: ROOT,
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    return out === VERSION;
  } catch {
    return false; // E404 / no such version → does not exist. (npm publish is the ultimate overwrite guard.)
  }
};
const existing = ALL_PKGS.filter(versionExists);
if (existing.length > 0) {
  fail(
    `These packages already publish ${VERSION} (versions are append-only):\n  ` +
      existing.join("\n  ") +
      `\nBump the version (new tag) and retry.`
  );
}

// ---- auth (skipped for dry-run) ----------------------------------------------
if (!DRY_RUN && !process.env.NODE_AUTH_TOKEN && !fs.existsSync(path.join(os.homedir(), ".npmrc"))) {
  fail(
    `No npm auth found. Put 'NODE_AUTH_TOKEN=<npm automation token>' in ${CONFIG_PATH}\n` +
      `(or set the env var, or run 'npm login').`
  );
}

// ---- build + assemble all 7 packages into dist/npm ---------------------------
const STAGE = path.join(ROOT, "dist", "npm");
fs.rmSync(STAGE, { recursive: true, force: true });
fs.mkdirSync(STAGE, { recursive: true });

// npm reads .npmrc from each package's OWN directory (its localPrefix) — a
// parent dir's .npmrc is ignored because every staged package has its own
// package.json. So drop one .npmrc into EACH package dir right before publishing.
// The token is referenced (${VAR}), never inlined, and npm always excludes
// .npmrc from the published tarball, so this is strictly publish-time auth.
const NPMRC = `registry=${REGISTRY}\n//registry.npmjs.org/:_authToken=\${NODE_AUTH_TOKEN}\nalways-auth=true\n`;
const writeNpmrc = (dir) => {
  if (!DRY_RUN && process.env.NODE_AUTH_TOKEN) fs.writeFileSync(path.join(dir, ".npmrc"), NPMRC);
};

// Must track the module path in go.mod — `-X` on an unknown symbol is silently
// ignored by the linker, so a stale path here ships binaries with no version.
const ldflags = `-s -w -X github.com/volcengine/byted-supabase-cli/internal/utils.Version=${VERSION}`;
const commonPkgFields = {
  version: VERSION,
  license: "MIT",
  homepage: "https://github.com/volcengine/byted-supabase-cli",
  repository: { type: "git", url: "git+https://github.com/volcengine/byted-supabase-cli.git" },
  publishConfig: { access: "public", registry: REGISTRY },
};

const stageDirs = [];
for (const t of TARGETS) {
  const name = platformPkgName(t);
  const dir = path.join(STAGE, `cli-${t.os}-${t.cpu}`);
  fs.mkdirSync(path.join(dir, "bin"), { recursive: true });
  const outBin = path.join(dir, "bin", `${BINARY}${t.exe}`);

  info(`building ${t.goos}/${t.goarch} → ${name}`);
  run("go", ["build", "-trimpath", "-ldflags", ldflags, "-o", outBin, "main.go"], {
    env: { ...process.env, CGO_ENABLED: "0", GOOS: t.goos, GOARCH: t.goarch },
  });
  fs.chmodSync(outBin, 0o755);

  fs.writeFileSync(
    path.join(dir, "package.json"),
    JSON.stringify(
      {
        name,
        ...commonPkgFields,
        description: `Native byted-supabase-cli binary for ${t.os}-${t.cpu}`,
        os: [t.os],
        cpu: [t.cpu],
        // The binary is also declared as a bin so npm guarantees its +x bit on
        // install (belt-and-suspenders over tarball mode preservation). It only
        // ever links into a nested node_modules/.bin, never the global PATH.
        bin: { [`${BINARY}-${t.os}-${t.cpu}`]: `bin/${BINARY}${t.exe}` },
        files: ["bin/"],
      },
      null,
      2
    ) + "\n"
  );
  writeNpmrc(dir);
  stageDirs.push({ name, dir });
}

// Main package: copy the launcher/setup script, pin optionalDependencies to the exact version.
const mainDir = path.join(STAGE, "cli");
fs.mkdirSync(path.join(mainDir, "bin"), { recursive: true });
for (const f of ["cli.js", "install.js"]) {
  fs.copyFileSync(path.join(ROOT, "bin", f), path.join(mainDir, "bin", f));
}
for (const f of ["README.md", "LICENSE"]) {
  if (fs.existsSync(path.join(ROOT, f))) fs.copyFileSync(path.join(ROOT, f), path.join(mainDir, f));
}
const rootPkg = JSON.parse(fs.readFileSync(path.join(ROOT, "package.json"), "utf8"));
const mainPkg = {
  name: rootPkg.name,
  version: VERSION,
  description: rootPkg.description,
  ...commonPkgFields,
  bugs: rootPkg.bugs,
  author: rootPkg.author,
  type: rootPkg.type,
  engines: rootPkg.engines,
  bin: rootPkg.bin,
  files: ["bin/cli.js", "bin/install.js"],
  optionalDependencies: Object.fromEntries(TARGETS.map((t) => [platformPkgName(t), VERSION])),
};
fs.writeFileSync(path.join(mainDir, "package.json"), JSON.stringify(mainPkg, null, 2) + "\n");
writeNpmrc(mainDir);

// ---- publish: platform packages FIRST, main package LAST ---------------------
const publishArgs = DRY_RUN ? ["publish", "--dry-run"] : ["publish"];
for (const { name, dir } of stageDirs) {
  info(`publishing ${name}@${VERSION}${DRY_RUN ? " (dry-run)" : ""}`);
  run("npm", publishArgs, { cwd: dir });
}
info(`publishing ${MAIN_PKG}@${VERSION}${DRY_RUN ? " (dry-run)" : ""}`);
run("npm", publishArgs, { cwd: mainDir });

console.log(
  `\n✓ ${
    DRY_RUN
      ? `Dry-run complete — would publish ${MAIN_PKG}@${VERSION} + ${TARGETS.length} platform packages`
      : `Published ${MAIN_PKG}@${VERSION} + ${TARGETS.length} platform packages`
  }.`
);
