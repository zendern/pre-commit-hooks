package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo makes an empty repository in a temporary directory and makes it the
// working directory, with no global config and no SKIP set.
func newRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("SKIP", "")
	t.Chdir(t.TempDir())
	gitRun(t, "init", "-q")
	gitRun(t, "config", "user.name", "Test")
	gitRun(t, "config", "user.email", "test@example.com")
	gitRun(t, "config", "commit.gpgsign", "false")
}

func gitRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// stagedMessage stages one file and writes msg to a commit message file.
func stagedMessage(t *testing.T, msg string) string {
	t.Helper()
	if err := os.WriteFile("a.txt", []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "add", "a.txt")
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunRecordsIndexTree(t *testing.T) {
	newRepo(t)
	path := stagedMessage(t, "feat: a\n")
	tree := gitRun(t, "write-tree")
	var stderr bytes.Buffer

	if code := run([]string{path}, &stderr); code != 0 {
		t.Fatalf("code %d, stderr %q, want 0", code, stderr.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Verified-tree: " + tree; !strings.Contains(string(got), want) {
		t.Errorf("message %q, want it to contain %q", got, want)
	}
}

func TestRunSkipLeavesCommitUnrecorded(t *testing.T) {
	newRepo(t)
	path := stagedMessage(t, "feat: a\n")
	t.Setenv("SKIP", "golangci-lint")
	var stderr bytes.Buffer

	if code := run([]string{path}, &stderr); code != 0 {
		t.Fatalf("code %d, want 0 so the commit still goes ahead", code)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "feat: a\n" {
		t.Errorf("message %q, want it unchanged", got)
	}
	if !strings.Contains(stderr.String(), "SKIP=golangci-lint") {
		t.Errorf("stderr %q, want a note naming SKIP", stderr.String())
	}
}

func TestRunNeedsOneMessageFile(t *testing.T) {
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 2 {
		t.Errorf("no argument: code %d, want 2", code)
	}
	if code := run([]string{"a", "b"}, &stderr); code != 2 {
		t.Errorf("two arguments: code %d, want 2", code)
	}
}

func TestRunFailsOutsideRepository(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("SKIP", "")
	t.Chdir(t.TempDir())
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte("feat: a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	code := run([]string{path}, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "verify-commits-record: ") {
		t.Fatalf("code %d, stderr %q; want 1 and a verify-commits-record error", code, stderr.String())
	}
}
