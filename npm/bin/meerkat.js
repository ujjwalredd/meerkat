#!/usr/bin/env node
// Meerkat npm wrapper.
//
//   npx meerkat-cli@latest init wizard
//   npx meerkat-cli@latest mcp start
//   npm install -g meerkat-cli
//
// On first run, downloads the Go binary for the host OS/arch from the
// GitHub release matching this package version. Caches in
// ~/.meerkat/bin/meerkat-<version>-<os>-<arch>[.exe]. Subsequent runs
// just exec the cached binary with the same argv.
//
// Falls back to `go install` if the host has Go but no matching release
// asset exists yet.

const { spawnSync } = require("child_process");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");

const PKG = require("../package.json");
const REPO = "ujjwalredd/meerkat";
const VERSION = "v" + PKG.version;

function plat() {
  const a = os.arch();
  const arch = a === "x64" ? "amd64" : a === "arm64" ? "arm64" : a;
  const p = os.platform();
  let osName = p;
  if (p === "win32") osName = "windows";
  const ext = osName === "windows" ? ".exe" : "";
  return { osName, arch, ext };
}

function cacheDir() {
  const d = path.join(os.homedir(), ".meerkat", "bin");
  fs.mkdirSync(d, { recursive: true });
  return d;
}

function cachedBinaryPath() {
  const { osName, arch, ext } = plat();
  return path.join(cacheDir(), `meerkat-${PKG.version}-${osName}-${arch}${ext}`);
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const f = fs.createWriteStream(dest);
    https
      .get(url, { headers: { "User-Agent": "meerkat-cli-installer" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          f.close();
          fs.unlinkSync(dest);
          return resolve(download(res.headers.location, dest));
        }
        if (res.statusCode !== 200) {
          f.close();
          fs.unlinkSync(dest);
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        res.pipe(f);
        f.on("finish", () => f.close(() => resolve(dest)));
      })
      .on("error", (e) => {
        f.close();
        try { fs.unlinkSync(dest); } catch (_) {}
        reject(e);
      });
  });
}

async function ensureBinary() {
  const bin = cachedBinaryPath();
  if (fs.existsSync(bin)) return bin;
  const { osName, arch, ext } = plat();
  const asset = `meerkat-${osName}-${arch}${ext}`;
  const url = `https://github.com/${REPO}/releases/download/${VERSION}/${asset}`;
  process.stderr.write(`meerkat: downloading ${url}\n`);
  try {
    await download(url, bin);
    fs.chmodSync(bin, 0o755);
    return bin;
  } catch (e) {
    process.stderr.write(`meerkat: download failed (${e.message})\n`);
    // Fallback: build from source if Go is installed.
    const goCheck = spawnSync("go", ["version"], { stdio: "ignore" });
    if (goCheck.status === 0) {
      process.stderr.write("meerkat: falling back to: go install\n");
      const r = spawnSync(
        "go",
        ["install", `github.com/${REPO}/cmd/meerkat@${VERSION}`],
        { stdio: "inherit", env: { ...process.env, GOBIN: cacheDir() } }
      );
      if (r.status === 0) {
        const goBin = path.join(cacheDir(), `meerkat${ext}`);
        if (fs.existsSync(goBin)) {
          fs.renameSync(goBin, bin);
          return bin;
        }
      }
    }
    throw new Error(
      `Could not obtain meerkat binary for ${osName}/${arch}. ` +
        `Install Go 1.22+ and retry, or grab a binary from ` +
        `https://github.com/${REPO}/releases`
    );
  }
}

async function main() {
  const argv = process.argv.slice(2);
  // postinstall hook: download eagerly so first real invocation is fast
  if (argv.length === 1 && argv[0] === "--install-binary-only") {
    try { await ensureBinary(); } catch (_) { /* best-effort */ }
    return;
  }
  const bin = await ensureBinary();
  const r = spawnSync(bin, argv, { stdio: "inherit" });
  process.exit(r.status === null ? 1 : r.status);
}

main().catch((e) => {
  process.stderr.write(`meerkat: ${e.message}\n`);
  process.exit(1);
});
