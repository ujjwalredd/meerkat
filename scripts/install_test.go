package scripts

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptInstallsPrebuiltAsset(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh integration tests require bash")
	}
	releaseRoot := t.TempDir()
	installDir := t.TempDir()
	version := "v9.9.9"
	asset := installerAssetName(runtime.GOOS, runtime.GOARCH)
	writeFakeRelease(t, releaseRoot, version, asset, fakeMeerkatBinary(), true)

	server := httptest.NewServer(http.FileServer(http.Dir(releaseRoot)))
	defer server.Close()

	out, err := runInstaller(t, installDir, version, server.URL)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "verified checksum for "+asset) {
		t.Fatalf("installer did not verify checksum:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(installDir, "meerkat")); err != nil {
		t.Fatalf("expected installed binary: %v", err)
	}
}

func TestInstallScriptRejectsChecksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh integration tests require bash")
	}
	releaseRoot := t.TempDir()
	installDir := t.TempDir()
	version := "v9.9.9"
	asset := installerAssetName(runtime.GOOS, runtime.GOARCH)
	writeFakeRelease(t, releaseRoot, version, asset, fakeMeerkatBinary(), false)

	server := httptest.NewServer(http.FileServer(http.Dir(releaseRoot)))
	defer server.Close()

	out, err := runInstaller(t, installDir, version, server.URL)
	if err == nil {
		t.Fatalf("install.sh succeeded despite checksum mismatch:\n%s", out)
	}
	if !strings.Contains(out, "checksum mismatch for "+asset) {
		t.Fatalf("missing checksum mismatch error:\n%s", out)
	}
}

func runInstaller(t *testing.T, installDir, version, releaseBaseURL string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "install.sh")
	cmd.Env = append(os.Environ(),
		"INSTALL_DIR="+installDir,
		"MEERKAT_VERSION="+version,
		"MEERKAT_RELEASE_BASE_URL="+releaseBaseURL,
		"MEERKAT_INSTALL_NO_GO_FALLBACK=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeFakeRelease(t *testing.T, root, version, asset string, content []byte, validChecksum bool) {
	t.Helper()
	releaseDir := filepath.Join(root, version)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetPath := filepath.Join(releaseDir, asset)
	if err := os.WriteFile(assetPath, content, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	expected := fmt.Sprintf("%x", sum)
	if !validChecksum {
		expected = strings.Repeat("0", 64)
	}
	checksums := fmt.Sprintf("%s  %s\n", expected, asset)
	if err := os.WriteFile(filepath.Join(releaseDir, "checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeMeerkatBinary() []byte {
	return []byte(`#!/usr/bin/env sh
if [ "$1" = "version" ]; then
  echo "meerkat test"
fi
`)
}

func installerAssetName(goos, goarch string) string {
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("meerkat-%s-%s%s", goos, goarch, ext)
}
