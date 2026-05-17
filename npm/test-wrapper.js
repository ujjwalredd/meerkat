const assert = require("assert");
const path = require("path");

const {
  plat,
  releaseAssetName,
  releaseURL,
  cachedBinaryPath,
} = require("./bin/meerkat.js");

assert.deepStrictEqual(plat("darwin", "x64"), {
  osName: "darwin",
  arch: "amd64",
  ext: "",
});
assert.deepStrictEqual(plat("linux", "arm64"), {
  osName: "linux",
  arch: "arm64",
  ext: "",
});
assert.deepStrictEqual(plat("win32", "x64"), {
  osName: "windows",
  arch: "amd64",
  ext: ".exe",
});

assert.strictEqual(
  releaseAssetName({ osName: "windows", arch: "amd64", ext: ".exe" }),
  "meerkat-windows-amd64.exe",
);
assert.strictEqual(
  releaseURL("owner/repo", "v1.2.3", "meerkat-linux-amd64"),
  "https://github.com/owner/repo/releases/download/v1.2.3/meerkat-linux-amd64",
);

const cached = cachedBinaryPath("/tmp/meerkat-home", {
  osName: "linux",
  arch: "amd64",
  ext: "",
}, false);
assert.strictEqual(
  cached,
  path.join("/tmp/meerkat-home", ".meerkat", "bin", "meerkat-0.4.0-linux-amd64"),
);

console.log("npm wrapper tests passed");
