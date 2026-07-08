#!/usr/bin/env node

// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

// Explicit setup flow for:
//
//   npx @byted-supabase/cli@latest install
//
// This is not a postinstall hook. It only runs when the user asks for setup,
// installs the CLI globally through npm, then lets the installed CLI manage its
// own skill so ~/.supabase/skills-state.json remains authoritative.
"use strict";

import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const PKG = "@byted-supabase/cli@latest";
const BINARY = "byted-supabase-cli";
const isWindows = process.platform === "win32";

function run(command, args, options = {}) {
  const actualCommand = isWindows ? "cmd.exe" : command;
  const actualArgs = isWindows ? ["/d", "/s", "/c", command, ...args] : args;
  const result = spawnSync(actualCommand, actualArgs, { stdio: "inherit", ...options });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    const err = new Error(`${command} exited with status ${result.status ?? 1}`);
    err.status = result.status ?? 1;
    throw err;
  }
}

function capture(command, args) {
  const actualCommand = isWindows ? "cmd.exe" : command;
  const actualArgs = isWindows ? ["/d", "/s", "/c", command, ...args] : args;
  const result = spawnSync(actualCommand, actualArgs, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} exited with status ${result.status ?? 1}`);
  return result.stdout.trim();
}

function globalBinaryPath() {
  const prefix = capture("npm", ["prefix", "-g"]);
  return isWindows ? path.join(prefix, `${BINARY}.cmd`) : path.join(prefix, "bin", BINARY);
}

function main() {
  const force = process.argv.slice(2).includes("--force");

  console.error(`Installing ${PKG} globally ...`);
  try {
    run("npm", ["install", "-g", PKG]);
  } catch (err) {
    console.error(`\nFailed to install the CLI: ${err.message}`);
    console.error(`Retry with: npm install -g ${PKG}`);
    process.exit(1);
  }

  let binary;
  try {
    binary = globalBinaryPath();
    if (!fs.existsSync(binary)) throw new Error(`installed binary not found at ${binary}`);
  } catch (err) {
    console.error(`\nCLI installed, but its global binary could not be located: ${err.message}`);
    console.error(`Install the skill with: ${BINARY} skills install`);
    process.exit(1);
  }

  // Proactively populate the per-user binary cache now, in the terminal (no MCP
  // handshake timeout, shows progress). This puts the native binary in the
  // deterministic ~/.byted-supabase-cli/<version> spot before any GUI host (Cursor
  // / VS Code) spawns the launcher from a tree that can't resolve the optional dep.
  // Best-effort: the launcher self-heals on first use if this is skipped.
  console.error("\nPreparing native binary cache ...");
  try {
    run(binary, ["__warm-cache"]);
  } catch (err) {
    console.error(`(cache warm skipped: ${err.message})`);
  }

  console.error("\nInstalling byted-supabase agent skill ...");
  try {
    const args = ["skills", "install"];
    if (force) args.push("--force");
    run(binary, args);
  } catch (err) {
    console.error(`\nCLI installed, but skill installation failed: ${err.message}`);
    console.error(`Retry with: ${BINARY} skills install${force ? " --force" : ""}`);
    process.exit(1);
  }

  console.error("\nSetup complete. Run: byted-supabase-cli help");
}

main();
