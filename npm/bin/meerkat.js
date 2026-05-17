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
const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");

const PKG = require("../package.json");
const REPO = "ujjwalredd/meerkat";
const VERSION = "v" + PKG.version;

function plat(platform = os.platform(), architecture = os.arch()) {
  const a = architecture;
  const arch = a === "x64" ? "amd64" : a === "arm64" ? "arm64" : a;
  const p = platform;
  let osName = p;
  if (p === "win32") osName = "windows";
  const ext = osName === "windows" ? ".exe" : "";
  return { osName, arch, ext };
}

function releaseAssetName(platformInfo = plat()) {
  return `meerkat-${platformInfo.osName}-${platformInfo.arch}${platformInfo.ext}`;
}

function releaseURL(repo = REPO, version = VERSION, asset = releaseAssetName()) {
  return `https://github.com/${repo}/releases/download/${version}/${asset}`;
}

function cacheDir(home = os.homedir(), create = true) {
  const d = path.join(home, ".meerkat", "bin");
  if (create) {
    fs.mkdirSync(d, { recursive: true });
  }
  return d;
}

function cachedBinaryPath(home = os.homedir(), platformInfo = plat(), create = true) {
  const { osName, arch, ext } = platformInfo;
  return path.join(cacheDir(home, create), `meerkat-${PKG.version}-${osName}-${arch}${ext}`);
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

function downloadText(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { "User-Agent": "meerkat-cli-installer" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return resolve(downloadText(res.headers.location));
        }
        if (res.statusCode !== 200) {
          res.resume();
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        let body = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => { body += chunk; });
        res.on("end", () => resolve(body));
      })
      .on("error", reject);
  });
}

function sha256File(file) {
  const h = crypto.createHash("sha256");
  h.update(fs.readFileSync(file));
  return h.digest("hex");
}

async function verifyChecksum(file, asset, repo = REPO, version = VERSION) {
  const checksumsURL = releaseURL(repo, version, "checksums.txt");
  let body;
  try {
    body = await downloadText(checksumsURL);
  } catch (e) {
    if (process.env.MEERKAT_REQUIRE_CHECKSUM === "1" || process.env.MEERKAT_REQUIRE_CHECKSUM === "true") {
      throw new Error(`checksums.txt unavailable: ${e.message}`);
    }
    process.stderr.write(`meerkat: warning: checksums.txt unavailable (${e.message}); continuing without checksum verification\n`);
    return;
  }
  const line = body.split(/\r?\n/).find((l) => l.trim().split(/\s+/)[1] === asset);
  if (!line) {
    throw new Error(`checksums.txt does not contain ${asset}`);
  }
  const expected = line.trim().split(/\s+/)[0];
  const actual = sha256File(file);
  if (actual !== expected) {
    throw new Error(`checksum mismatch for ${asset}: expected ${expected}, got ${actual}`);
  }
  process.stderr.write(`meerkat: verified checksum for ${asset}\n`);
}

async function ensureBinary() {
  const platformInfo = plat();
  const bin = cachedBinaryPath(os.homedir(), platformInfo);
  if (fs.existsSync(bin)) return bin;
  const { osName, arch, ext } = platformInfo;
  const asset = releaseAssetName(platformInfo);
  const url = releaseURL(REPO, VERSION, asset);
  process.stderr.write(`meerkat: downloading ${url}\n`);
  try {
    await download(url, bin);
    await verifyChecksum(bin, asset);
    fs.chmodSync(bin, 0o755);
    return bin;
  } catch (e) {
    process.stderr.write(`meerkat: download failed (${e.message})\n`);
    try { fs.unlinkSync(bin); } catch (_) {}
    if (process.env.MEERKAT_INSTALL_NO_GO_FALLBACK === "1" || process.env.MEERKAT_INSTALL_NO_GO_FALLBACK === "true") {
      throw new Error("download failed and MEERKAT_INSTALL_NO_GO_FALLBACK=1");
    }
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

if (require.main === module) {
  main().catch((e) => {
    process.stderr.write(`meerkat: ${e.message}\n`);
    process.exit(1);
  });
}

module.exports = {
  plat,
  releaseAssetName,
  releaseURL,
  sha256File,
  cachedBinaryPath,
};
