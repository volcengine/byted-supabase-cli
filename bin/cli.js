#!/usr/bin/env node

// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// byted-supabase-cli launcher: find the platform-specific native binary and exec
// it. It is resolved from the optional dependency in node_modules, a per-user
// cache, or an on-demand `npm install` into that cache (set
// BYTED_SUPABASE_CLI_NO_AUTO_DOWNLOAD=1 to disable that download).
"use strict";

import { spawn, spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const args = process.argv.slice(2);
const SELF = fileURLToPath(import.meta.url);
const binDir = path.dirname(SELF);

const PLATFORM_PKG = `@byted-supabase/cli-${process.platform}-${process.arch}`;
const BIN_NAME = process.platform === "win32" ? "byted-supabase-cli.exe" : "byted-supabase-cli";
const PKG_BIN_REL = path.join(...PLATFORM_PKG.split("/"), "bin", BIN_NAME);
const NM_REL = process.platform === "win32" ? "node_modules" : path.join("lib", "node_modules");

const cacheRoot = (version) => path.join(os.homedir(), ".byted-supabase-cli", version);
const cachedBinary = (version) => path.join(cacheRoot(version), NM_REL, PKG_BIN_REL);

function launcherVersion() {
  try {
    return JSON.parse(fs.readFileSync(path.join(binDir, "..", "package.json"), "utf8")).version || null;
  } catch {
    return null;
  }
}

function resolveBinary() {
  const manifest = require.resolve(`${PLATFORM_PKG}/package.json`);
  return path.join(path.dirname(manifest), "bin", BIN_NAME);
}

// Copy a resolved binary into the per-user cache (hardlink, else copy) so later
// runs find it without re-resolving. Best-effort; skip if already present.
function mirrorToCache(resolvedBin, version) {
  if (!version) return;
  const target = cachedBinary(version);
  if (fs.existsSync(target)) return;
  try {
    fs.mkdirSync(path.dirname(target), { recursive: true });
    const tmp = `${target}.tmp-${process.pid}`;
    fs.rmSync(tmp, { force: true });
    try {
      fs.linkSync(resolvedBin, tmp);
    } catch {
      fs.copyFileSync(resolvedBin, tmp);
      try { fs.chmodSync(tmp, 0o755); } catch {}
    }
    fs.renameSync(tmp, target);
  } catch {
    /* best effort */
  }
}

function runNpmInstall(version, prefix, force) {
  const npmArgs = [
    "install", "-g", `${PLATFORM_PKG}@${version}`,
    "--prefix", prefix,
    `--os=${process.platform}`, `--cpu=${process.arch}`,
    "--ignore-scripts", "--no-audit", "--no-fund", "--loglevel=error",
  ];
  if (force) npmArgs.push("--force");
  // Prefer npm on PATH; fall back to the npm beside this node. stdout -> fd 2.
  const env = {
    ...process.env,
    PATH: `${process.env.PATH || ""}${path.delimiter}${path.dirname(process.execPath)}`,
  };
  const result =
    process.platform === "win32"
      ? spawnSync("cmd.exe", ["/d", "/s", "/c", "npm", ...npmArgs], { stdio: ["ignore", 2, 2], env })
      : spawnSync("npm", npmArgs, { stdio: ["ignore", 2, 2], env });
  return !result.error && result.status === 0;
}

// Install the platform package into the per-user cache. Throws on failure.
function downloadViaNpm(version) {
  const root = cacheRoot(version);
  const target = cachedBinary(version);
  const tmp = `${root}.tmp-${process.pid}`;
  fs.rmSync(tmp, { recursive: true, force: true });
  fs.mkdirSync(tmp, { recursive: true });
  if (!runNpmInstall(version, tmp, false) && !runNpmInstall(version, tmp, true)) {
    fs.rmSync(tmp, { recursive: true, force: true });
    throw new Error(`npm install ${PLATFORM_PKG}@${version} failed`);
  }
  const installed = path.join(tmp, NM_REL, PKG_BIN_REL);
  if (!fs.existsSync(installed)) {
    fs.rmSync(tmp, { recursive: true, force: true });
    throw new Error(`installed ${PLATFORM_PKG} did not contain ${BIN_NAME}`);
  }
  try { fs.chmodSync(installed, 0o755); } catch {}
  try {
    fs.renameSync(tmp, root);
  } catch (e) {
    fs.rmSync(tmp, { recursive: true, force: true });
    if (!fs.existsSync(target)) throw e;
  }
  return target;
}

function sleepSync(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function waitForTarget(target, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (fs.existsSync(target)) return true;
    sleepSync(500);
  }
  return fs.existsSync(target);
}

// Single-flight lock (mkdir is atomic), with stale recovery.
function acquireLock(version) {
  const lock = `${cacheRoot(version)}.lock`;
  try { fs.mkdirSync(path.dirname(lock), { recursive: true }); } catch {}
  try {
    fs.mkdirSync(lock);
    return lock;
  } catch (e) {
    if (e.code !== "EEXIST") return null;
    try {
      if (Date.now() - fs.statSync(lock).mtimeMs > 15 * 60 * 1000) {
        fs.rmSync(lock, { recursive: true, force: true });
        fs.mkdirSync(lock);
        return lock;
      }
    } catch {}
    return null;
  }
}

// Download into the cache once; concurrent callers wait for the in-flight one.
function runLockedDownload(version) {
  const target = cachedBinary(version);
  if (fs.existsSync(target)) return true;
  const lock = acquireLock(version);
  if (!lock) return waitForTarget(target, 15 * 60 * 1000);
  try {
    if (fs.existsSync(target)) return true;
    downloadViaNpm(version);
    return fs.existsSync(target);
  } finally {
    try { fs.rmSync(lock, { recursive: true, force: true }); } catch {}
  }
}

// Run the download in a detached process so it is not bound to this one's lifetime.
function startDetachedDownload(version) {
  try { fs.mkdirSync(path.dirname(cacheRoot(version)), { recursive: true }); } catch {}
  let out = "ignore";
  try { out = fs.openSync(`${cacheRoot(version)}.download.log`, "a"); } catch {}
  try {
    spawn(process.execPath, [SELF, "__heal-download", version], { detached: true, stdio: ["ignore", out, out] }).unref();
  } catch {}
}

// ---- subcommands the launcher handles itself ---------------------------------

if (args[0] === "install") {
  const result = spawnSync(process.execPath, [path.join(binDir, "install.js"), ...args.slice(1)], {
    stdio: "inherit",
  });
  if (result.error) {
    console.error(`[@byted-supabase/cli] ${result.error.message}`);
    process.exit(1);
  }
  process.exit(typeof result.status === "number" ? result.status : 1);
}

// Populate the per-user cache ahead of time (used by the install flow).
if (args[0] === "__warm-cache") {
  const v = launcherVersion();
  try {
    if (v && !fs.existsSync(cachedBinary(v))) {
      let resolved = null;
      try {
        const p = resolveBinary();
        if (fs.existsSync(p)) resolved = p;
      } catch {}
      if (resolved) {
        mirrorToCache(resolved, v);
      } else if (!process.env.BYTED_SUPABASE_CLI_NO_AUTO_DOWNLOAD) {
        runLockedDownload(v);
      }
      if (fs.existsSync(cachedBinary(v))) {
        console.error(`[@byted-supabase/cli] cached native binary for ${process.platform}-${process.arch}`);
      }
    }
  } catch (e) {
    console.error(`[@byted-supabase/cli] cache warm skipped: ${e && e.message}`);
  }
  process.exit(0);
}

// Worker for the detached download.
if (args[0] === "__heal-download") {
  const v = args[1] || launcherVersion();
  let ok = false;
  try {
    ok = !!v && runLockedDownload(v);
  } catch (e) {
    console.error(`[@byted-supabase/cli] background download failed: ${e && e.message}`);
  }
  process.exit(ok ? 0 : 1);
}

// ---- locate the binary, then exec it -----------------------------------------

const version = launcherVersion();

let binPath = null;
try {
  const p = resolveBinary();
  if (fs.existsSync(p)) {
    binPath = p;
    mirrorToCache(p, version);
  }
} catch {}

if (!binPath && version) {
  const c = cachedBinary(version);
  if (fs.existsSync(c)) binPath = c;
}

if (!binPath && version && !process.env.BYTED_SUPABASE_CLI_NO_AUTO_DOWNLOAD) {
  if (args[0] === "mcp" && args[1] === "serve") {
    // Don't block the MCP handshake: download in the background and exit; a later
    // reconnect uses the cached binary.
    startDetachedDownload(version);
    console.error(
      `[@byted-supabase/cli] native binary for ${process.platform}-${process.arch} not found; preparing it in the background.\n` +
        `Reconnect once it is ready, or run: npx @byted-supabase/cli@latest install`
    );
    process.exit(1);
  }
  console.error(`[@byted-supabase/cli] native binary not found; installing ${PLATFORM_PKG}@${version} ...`);
  try {
    if (runLockedDownload(version)) binPath = cachedBinary(version);
  } catch (e) {
    console.error(`[@byted-supabase/cli] auto-download failed: ${e.message}`);
  }
}

if (!binPath || !fs.existsSync(binPath)) {
  console.error(
    `[@byted-supabase/cli] Could not find or install the native binary for ${process.platform}-${process.arch}.\n` +
      `  - run setup: npx @byted-supabase/cli@latest install\n` +
      `  - or reinstall: npm install -g @byted-supabase/cli`
  );
  process.exit(1);
}

const result = spawnSync(binPath, args, { stdio: "inherit" });
if (result.error) {
  console.error(`[@byted-supabase/cli] ${result.error.message}`);
  process.exit(1);
}
process.exit(typeof result.status === "number" ? result.status : 1);
