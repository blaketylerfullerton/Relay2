package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cmdUpdate pulls the latest source, rebuilds, and replaces the running binary.
//
// It is the distribution story for V1: a solo operator pushes from a dev
// machine, then runs `relay update` on each box to bring it current. The flow
// is deliberately boring — git pull, go build, atomic rename — so there is no
// release infrastructure to babysit while the project churns.
//
// The source tree is RELAY_SRC, defaulting to ~/.relay/src (where the install
// script clones it). The binary that gets replaced is whatever is running now
// (os.Executable), so this works regardless of where it was installed.
func (a *App) cmdUpdate(args []string) int {
	force := false
	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			fmt.Fprintln(a.Out, "relay update — pull latest source, rebuild, and replace this binary")
			fmt.Fprintln(a.Out, "\n  --force, -f   rebuild even if already up to date")
			fmt.Fprintln(a.Out, "\nSource tree: $RELAY_SRC (default ~/.relay/src)")
			return 0
		default:
			fmt.Fprintf(a.Err, "relay: unknown flag %q for update\n", arg)
			return 2
		}
	}

	src, err := srcDir()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}
	if !isGitRepo(src) {
		fmt.Fprintf(a.Err, "relay: %s is not a git checkout.\n", src)
		fmt.Fprintf(a.Err, "  set RELAY_SRC to your Relay clone, or run the install script.\n")
		return 1
	}

	before := gitRev(src)

	fmt.Fprintf(a.Out, "Updating from %s ...\n", src)
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

	// Build next to the live binary so the final swap is a same-filesystem
	// rename — atomic, and safe to do while this process is running.
	exe, err := selfPath()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: cannot locate running binary: %v\n", err)
		return 1
	}
	staged := exe + ".new"

	fmt.Fprintln(a.Out, "  building...")
	if out, err := runIn(src, "go", "build", "-o", staged, "."); err != nil {
		fmt.Fprintf(a.Err, "relay: build failed: %v\n%s", err, out)
		return 1
	}

	if err := os.Chmod(staged, 0o755); err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		os.Remove(staged)
		return 1
	}
	if err := os.Rename(staged, exe); err != nil {
		fmt.Fprintf(a.Err, "relay: could not replace %s: %v\n", exe, err)
		os.Remove(staged)
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

// runIn runs a command in dir and returns its combined output.
func runIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	return string(out), err
}
