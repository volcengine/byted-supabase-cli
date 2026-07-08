// Copyright (c) 2026 ByteDance Ltd. and/or its affiliates
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const LAUNCHER = path.join(ROOT, "bin", "cli.js");

function writeExecutable(file, content) {
  fs.writeFileSync(file, content, { mode: 0o755 });
}

function fixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "byted-supabase-install-"));
  const fakeBin = path.join(dir, "fake-bin");
  const prefix = path.join(dir, "prefix");
  const log = path.join(dir, "calls.log");
  fs.mkdirSync(fakeBin, { recursive: true });
  fs.mkdirSync(path.join(prefix, "bin"), { recursive: true });

  writeExecutable(
    path.join(fakeBin, "npm"),
    `#!/bin/sh
echo "npm $*" >> "$INSTALL_TEST_LOG"
if [ "$1 $2" = "prefix -g" ]; then
  printf '%s\\n' "$INSTALL_TEST_PREFIX"
  exit 0
fi
exit "\${INSTALL_TEST_NPM_STATUS:-0}"
`
  );
  writeExecutable(
    path.join(prefix, "bin", "byted-supabase-cli"),
    `#!/bin/sh
echo "cli $*" >> "$INSTALL_TEST_LOG"
exit "\${INSTALL_TEST_SKILLS_STATUS:-0}"
`
  );

  return {
    dir,
    log,
    env: {
      ...process.env,
      PATH: `${fakeBin}:${process.env.PATH}`,
      INSTALL_TEST_LOG: log,
      INSTALL_TEST_PREFIX: prefix,
    },
  };
}

function calls(log) {
  return fs.existsSync(log) ? fs.readFileSync(log, "utf8").trim().split("\n") : [];
}

test("launcher install globally installs CLI then installs skill", { skip: process.platform === "win32" }, () => {
  const f = fixture();
  const result = spawnSync(process.execPath, [LAUNCHER, "install"], {
    env: f.env,
    encoding: "utf8",
  });

  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(calls(f.log), [
    "npm install -g @byted-supabase/cli@latest",
    "npm prefix -g",
    "cli __warm-cache",
    "cli skills install",
  ]);
});

test("launcher install forwards --force to skills install", { skip: process.platform === "win32" }, () => {
  const f = fixture();
  execFileSync(process.execPath, [LAUNCHER, "install", "--force"], { env: f.env });

  assert.equal(calls(f.log).at(-1), "cli skills install --force");
});

test("launcher install stops before skills when global npm install fails", { skip: process.platform === "win32" }, () => {
  const f = fixture();
  const result = spawnSync(process.execPath, [LAUNCHER, "install"], {
    env: { ...f.env, INSTALL_TEST_NPM_STATUS: "23" },
    encoding: "utf8",
  });

  assert.equal(result.status, 1);
  assert.deepEqual(calls(f.log), ["npm install -g @byted-supabase/cli@latest"]);
  assert.match(result.stderr, /Failed to install the CLI/);
});

// ---- self-heal (missing native binary repaired via `npm install`) -----------

const PLATFORM_PKG = `@byted-supabase/cli-${process.platform}-${process.arch}`;

// A fake `npm` that mimics `install -g <pkg> --prefix <dir>` by materializing the
// platform package's binary under the prefix (POSIX `lib/node_modules` layout),
// so the launcher's self-heal can find and exec it. Redirects the cache HOME so
// the real ~/.byted-supabase-cli is never touched.
function healFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "byted-supabase-heal-"));
  const fakeBin = path.join(dir, "fake-bin");
  const home = path.join(dir, "home");
  const log = path.join(dir, "npm-calls.log");
  fs.mkdirSync(fakeBin, { recursive: true });
  fs.mkdirSync(home, { recursive: true });

  writeExecutable(
    path.join(fakeBin, "npm"),
    `#!/bin/sh
echo "npm $*" >> "$HEAL_TEST_LOG"
prefix=""
pkgspec=""
while [ $# -gt 0 ]; do
  case "$1" in
    --prefix) prefix="$2"; shift 2 ;;
    @byted-supabase/*) pkgspec="$1"; shift ;;
    *) shift ;;
  esac
done
[ -n "$prefix" ] && [ -n "$pkgspec" ] || exit 1
[ "\${HEAL_TEST_NPM_STATUS:-0}" = "0" ] || exit "$HEAL_TEST_NPM_STATUS"
dest="$prefix/lib/node_modules/\${pkgspec%@*}/bin"
mkdir -p "$dest"
printf '#!/bin/sh\\necho "NATIVE-RAN $*"\\n' > "$dest/byted-supabase-cli"
chmod +x "$dest/byted-supabase-cli"
exit 0
`
  );

  return {
    home,
    log,
    env: {
      ...process.env,
      PATH: `${fakeBin}:${process.env.PATH}`,
      HOME: home,
      HEAL_TEST_LOG: log,
    },
  };
}

test("launcher self-heals a missing native binary via npm install", { skip: process.platform === "win32" }, () => {
  const f = healFixture();
  const result = spawnSync(process.execPath, [LAUNCHER, "help"], { env: f.env, encoding: "utf8" });

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /NATIVE-RAN help/); // execed the healed binary, forwarding args
  const log = calls(f.log);
  assert.equal(log.length, 1, `expected exactly one npm call, got:\n${log.join("\n")}`);
  assert.match(log[0], /^npm install -g @byted-supabase\/cli-\S+ --prefix \S+ --os=\S+ --cpu=\S+/);
});

test("launcher self-heal is disabled by BYTED_SUPABASE_CLI_NO_AUTO_DOWNLOAD", { skip: process.platform === "win32" }, () => {
  const f = healFixture();
  const result = spawnSync(process.execPath, [LAUNCHER, "help"], {
    env: { ...f.env, BYTED_SUPABASE_CLI_NO_AUTO_DOWNLOAD: "1" },
    encoding: "utf8",
  });

  assert.equal(result.status, 1);
  assert.deepEqual(calls(f.log), []); // npm never invoked
  assert.match(result.stderr, /Could not find or install the native binary/);
});

test("launcher reuses a cached native binary without invoking npm", { skip: process.platform === "win32" }, () => {
  const f = healFixture();
  const version = JSON.parse(fs.readFileSync(path.join(ROOT, "package.json"), "utf8")).version;
  const dest = path.join(
    f.home,
    ".byted-supabase-cli",
    version,
    "lib",
    "node_modules",
    PLATFORM_PKG,
    "bin",
    "byted-supabase-cli"
  );
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  writeExecutable(dest, `#!/bin/sh\necho "CACHED-RAN $*"\n`);

  const result = spawnSync(process.execPath, [LAUNCHER, "version"], { env: f.env, encoding: "utf8" });

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /CACHED-RAN version/);
  assert.deepEqual(calls(f.log), []); // cache hit → npm not called
});

// ---- proactive cache warming (no scanning, no reactive download) ------------

// A staged "installed package": a copy of the real launcher with its own sibling
// optional dependency, so require.resolve succeeds (as it would in a healthy
// install). A fake `npm` on PATH lets us assert it is NOT invoked. HOME is
// redirected so the real ~/.byted-supabase-cli is untouched.
function stagedFixture(version, nativeEcho) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "byted-supabase-staged-"));
  const fakeBin = path.join(dir, "fake-bin");
  const home = path.join(dir, "home");
  const log = path.join(dir, "npm-calls.log");
  const pkg = path.join(dir, "pkg");
  fs.mkdirSync(fakeBin, { recursive: true });
  fs.mkdirSync(home, { recursive: true });
  writeExecutable(path.join(fakeBin, "npm"), `#!/bin/sh\necho "npm $*" >> "$HEAL_TEST_LOG"\nexit 0\n`);

  fs.mkdirSync(path.join(pkg, "bin"), { recursive: true });
  fs.copyFileSync(LAUNCHER, path.join(pkg, "bin", "cli.js"));
  fs.writeFileSync(
    path.join(pkg, "package.json"),
    JSON.stringify({ name: "@byted-supabase/cli", version, type: "module", bin: { "byted-supabase-cli": "bin/cli.js" } })
  );
  const dep = path.join(pkg, "node_modules", "@byted-supabase", `cli-${process.platform}-${process.arch}`);
  fs.mkdirSync(path.join(dep, "bin"), { recursive: true });
  fs.writeFileSync(path.join(dep, "package.json"), JSON.stringify({ name: PLATFORM_PKG, version }));
  writeExecutable(path.join(dep, "bin", "byted-supabase-cli"), `#!/bin/sh\necho "${nativeEcho} $*"\n`);

  return {
    home,
    log,
    cli: path.join(pkg, "bin", "cli.js"),
    env: { ...process.env, PATH: `${fakeBin}:${process.env.PATH}`, HOME: home, HEAL_TEST_LOG: log },
  };
}

const cachedBinaryPath = (home, version) =>
  path.join(home, ".byted-supabase-cli", version, "lib", "node_modules", PLATFORM_PKG, "bin", "byted-supabase-cli");

test("launcher mirrors a resolved binary into the per-user cache (no npm)", { skip: process.platform === "win32" }, () => {
  const version = "9.9.9";
  const f = stagedFixture(version, "RESOLVED-RAN");

  const result = spawnSync(process.execPath, [f.cli, "help"], { env: f.env, encoding: "utf8" });

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /RESOLVED-RAN help/); // execed the resolved optional-dep binary
  assert.ok(fs.existsSync(cachedBinaryPath(f.home, version)), "resolved binary should be mirrored into the cache");
  assert.deepEqual(calls(f.log), []); // mirror is a hardlink/copy — npm never invoked
});

test("mcp serve does not block on a missing binary; downloads in the background", { skip: process.platform === "win32" }, async () => {
  const f = healFixture();
  const version = JSON.parse(fs.readFileSync(path.join(ROOT, "package.json"), "utf8")).version;

  const result = spawnSync(process.execPath, [LAUNCHER, "mcp", "serve"], { env: f.env, encoding: "utf8" });

  assert.equal(result.status, 1); // fast exit — did not hang waiting for the download
  assert.doesNotMatch(result.stdout, /NATIVE-RAN/); // did not exec a binary
  assert.match(result.stderr, /background/i); // told the user it is downloading in the background

  // The detached worker outlives our exit and populates the cache.
  const cached = cachedBinaryPath(f.home, version);
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline && !fs.existsSync(cached)) {
    await new Promise((r) => setTimeout(r, 200));
  }
  assert.ok(fs.existsSync(cached), "detached background download should populate the cache");
});

test("__warm-cache downloads the binary into the cache when the optional dep is absent", { skip: process.platform === "win32" }, () => {
  const f = healFixture();
  const version = JSON.parse(fs.readFileSync(path.join(ROOT, "package.json"), "utf8")).version;

  const result = spawnSync(process.execPath, [LAUNCHER, "__warm-cache"], { env: f.env, encoding: "utf8" });

  assert.equal(result.status, 0, result.stderr); // best-effort: never fails its caller
  assert.ok(fs.existsSync(cachedBinaryPath(f.home, version)), "warm should populate the cache");
  assert.equal(calls(f.log).length, 1, `warm downloads once via npm, got:\n${calls(f.log).join("\n")}`);
});
