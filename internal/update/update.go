// Package update implements `sshit upgrade`: it downloads the latest release
// asset for the current platform and atomically replaces the running binary.
// The asset names and mirror fallback mirror docs/public/install.sh and
// docs/public/install.ps1.
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	repoOwner    = "oboard"
	repoName     = "sshit"
	mirrorPrefix = "https://ghfast.top/"
	timeout      = 30 * time.Second
)

// assetName returns the release asset name for the current platform, matching
// the Release workflow build matrix and docs/public/install.sh + install.ps1.
func assetName() (string, error) {
	var osName string
	switch runtime.GOOS {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "macos"
	case "windows":
		osName = "windows"
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	if runtime.GOOS == "windows" {
		return fmt.Sprintf("sshit-windows-%s.exe", arch), nil
	}
	return fmt.Sprintf("sshit-%s-%s", osName, arch), nil
}

// checkUnsupported fails fast for platforms the release matrix doesn't cover.
func checkUnsupported() error {
	_, err := assetName()
	return err
}

// LatestVersion queries GitHub's releases/latest endpoint and returns the tag
// name (e.g. "v0.1.4").
func LatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("query latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query latest release: HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	return rel.TagName, nil
}

// replaceSelf replaces the running executable with the bytes read from src.
// It first writes to a temp file next to the executable, then performs an
// atomic rename: POSIX renames over the live binary directly (the running
// process keeps its old inode until exit); Windows must move the locked
// executable aside first.
func replaceSelf(src io.Reader) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".sshit-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	_ = os.Chmod(tmpName, 0o755)
	if f, err := os.Open(tmpName); err == nil {
		_ = f.Sync()
		_ = f.Close()
	}

	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return fmt.Errorf("move current binary aside: %w", err)
		}
		if err := os.Rename(tmpName, exe); err != nil {
			return fmt.Errorf("install new binary: %w", err)
		}
		_ = os.Remove(old)
		return nil
	}

	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// downloadToTemp downloads an asset into a fresh temp file under destDir and
// returns its path. It tries GitHub directly first, then the mirror; if both
// fail it returns an error. The temp file sits in the same directory as the
// target binary so the later rename is atomic.
func downloadToTemp(asset, destDir string) (string, error) {
	direct := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/%s", repoOwner, repoName, asset)
	mirror := mirrorPrefix + direct
	client := &http.Client{Timeout: timeout}

	tmp, err := os.CreateTemp(destDir, ".sshit-download-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// fetch tries one source from a clean offset; on success rewinds and
	// returns nil so a later source can overwrite any partial bytes.
	fetch := func(u string) error {
		resp, err := client.Get(u)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return err
		}
		n, err := io.Copy(tmp, resp.Body)
		if err != nil {
			return err
		}
		return tmp.Truncate(n)
	}

	if err := fetch(direct); err != nil {
		fmt.Printf("upgrade: direct download failed (%v); trying mirror...\n", err)
		if merr := fetch(mirror); merr != nil {
			tmp.Close()
			return "", fmt.Errorf("download %s: %v", asset, merr)
		}
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return tmpName, nil
}

// Run executes the upgrade subcommand with the current build version, and
// returns the process exit code.
//   - "upgrade"         download latest and replace self
//   - "upgrade --check"  print current/latest versions
func Run(buildVersion string, args []string) int {
	// Fail fast on unsupported platforms.
	if err := checkUnsupported(); err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
		return 1
	}

	if len(args) >= 1 && args[0] == "--check" {
		latest, err := LatestVersion()
		if err != nil {
			fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
			return 1
		}
		fmt.Printf("%s (current) -> %s (latest)\n", buildVersion, latest)
		return 0
	}

	asset, err := assetName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
		return 1
	}

	var latest string
	if l, err := LatestVersion(); err == nil {
		latest = l
	} else {
		latest = "latest"
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: locate self: %v\n", err)
		return 1
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: resolve executable: %v\n", err)
		return 1
	}

	tmpName, err := downloadToTemp(asset, filepath.Dir(exe))
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: download %s: %v\n", asset, err)
		return 1
	}
	defer os.Remove(tmpName)

	f, err := os.Open(tmpName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
		return 1
	}
	if err := replaceSelf(f); err != nil {
		f.Close()
		fmt.Fprintf(os.Stderr, "upgrade: %v\n", err)
		return 1
	}
	f.Close()

	fmt.Printf("upgraded to %s. Restart to use the new version.\n", latest)
	return 0
}
