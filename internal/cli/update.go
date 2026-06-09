package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// repoSlug is the GitHub owner/name used to fetch prebuilt release binaries.
const repoSlug = "blaketylerfullerton/Relay2"

// Version is stamped at build time via -ldflags for release binaries; it stays
// "dev" for local `go build`. The release workflow sets it to the git tag.
var Version = "dev"

// cmdUpdate brings the running binary current. It has two modes:
//
//   - source mode: on a machine with a Relay git checkout ($RELAY_SRC) and Go
//     installed, it does git pull + go build (your dev machines).
//   - release mode: everywhere else, it downloads the prebuilt binary for this
//     OS/arch from the latest GitHub Release — no Go, no source tree.
//
// Either way the swap is an atomic rename over the live binary, so this works
// regardless of where the binary was installed.
func (a *App) cmdUpdate(args []string) int {
	force := false
	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			fmt.Fprintln(a.Out, "relay update — bring this binary up to date")
			fmt.Fprintln(a.Out, "\n  --force, -f   reinstall even if already up to date")
			fmt.Fprintln(a.Out, "\nWith a git checkout at $RELAY_SRC it rebuilds from source;")
			fmt.Fprintln(a.Out, "otherwise it downloads the latest release binary.")
			return 0
		default:
			fmt.Fprintf(a.Err, "relay: unknown flag %q for update\n", arg)
			return 2
		}
	}

	exe, err := selfPath()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: cannot locate running binary: %v\n", err)
		return 1
	}

	src, _ := srcDir()
	if isGitRepo(src) && haveGo() {
		return a.updateFromSource(src, exe, force)
	}
	return a.updateFromRelease(exe, force)
}

// updateFromSource pulls the checkout and rebuilds in place.
func (a *App) updateFromSource(src, exe string, force bool) int {
	before := gitRev(src)

	fmt.Fprintf(a.Out, "Updating from source (%s) ...\n", src)
	fmt.Fprintln(a.Out, "  pulling latest source...")
	if out, err := runIn(src, "git", "pull", "--ff-only"); err != nil {
		fmt.Fprintf(a.Err, "relay: git pull failed: %v\n%s", err, out)
		return 1
	}

	after := gitRev(src)
	if before == after && !force {
		fmt.Fprintf(a.Out, "Already up to date (%s).\n", short(after))
		return 0
	}

	// Build next to the live binary so the final swap is a same-filesystem,
	// atomic rename — safe to do while this process is running.
	staged := exe + ".new"
	fmt.Fprintln(a.Out, "  building...")
	if out, err := runIn(src, "go", "build", "-o", staged, "."); err != nil {
		fmt.Fprintf(a.Err, "relay: build failed: %v\n%s", err, out)
		return 1
	}
	if err := swap(staged, exe); err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}

	if before == after {
		fmt.Fprintf(a.Out, "Rebuilt %s (no source change).\n", short(after))
	} else {
		fmt.Fprintf(a.Out, "Updated %s -> %s.\n", short(before), short(after))
	}
	fmt.Fprintf(a.Out, "Installed to %s\n", exe)
	return 0
}

// updateFromRelease downloads the latest prebuilt binary for this OS/arch and
// swaps it in. No Go toolchain or source checkout required.
func (a *App) updateFromRelease(exe string, force bool) int {
	asset := fmt.Sprintf("relay_%s_%s", runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repoSlug, asset)

	fmt.Fprintf(a.Out, "Checking for the latest release (%s/%s) ...\n", runtime.GOOS, runtime.GOARCH)
	staged := exe + ".new"
	if err := download(url, staged); err != nil {
		fmt.Fprintf(a.Err, "relay: download failed: %v\n", err)
		fmt.Fprintf(a.Err, "  (no prebuilt binary for %s? publish a release, or install Go to build from source.)\n", asset)
		return 1
	}

	newVer := binaryVersion(staged)
	if newVer != "" && newVer == Version && !force {
		os.Remove(staged)
		fmt.Fprintf(a.Out, "Already up to date (%s).\n", Version)
		return 0
	}
	if err := swap(staged, exe); err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}

	fmt.Fprintf(a.Out, "Updated %s -> %s.\n", Version, dflt(newVer, "latest"))
	fmt.Fprintf(a.Out, "Installed to %s\n", exe)
	return 0
}

// download fetches url to dst (an executable), following redirects.
func download(url, dst string) error {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dst)
		return err
	}
	return f.Close()
}

// swap makes staged executable and atomically renames it over exe.
func swap(staged, exe string) error {
	if err := os.Chmod(staged, 0o755); err != nil {
		os.Remove(staged)
		return err
	}
	if err := os.Rename(staged, exe); err != nil {
		os.Remove(staged)
		return fmt.Errorf("could not replace %s: %w", exe, err)
	}
	return nil
}

// binaryVersion runs `<path> version` to read a downloaded binary's version.
func binaryVersion(path string) string {
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func haveGo() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// srcDir resolves the Relay source checkout: $RELAY_SRC, else ~/.relay/src.
func srcDir() (string, error) {
	if s := os.Getenv("RELAY_SRC"); s != "" {
		return s, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home dir: %w", err)
	}
	return filepath.Join(home, ".relay", "src"), nil
}

// selfPath is the on-disk path of the running binary, with symlinks resolved so
// the rename replaces the real file rather than dangling a link.
func selfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved, nil
	}
	return exe, nil
}

func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

func gitRev(dir string) string {
	out, err := runIn(dir, "git", "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func short(rev string) string {
	if len(rev) >= 7 {
		return rev[:7]
	}
	if rev == "" {
		return "unknown"
	}
	return rev
}

func dflt(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// runIn runs a command in dir and returns its combined output.
func runIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	return string(out), err
}
