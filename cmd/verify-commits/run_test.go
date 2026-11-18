package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zendern/pre-commit-hooks/internal/commitverify"
)

// newRepo makes an empty repository in a temporary directory and makes it the
// working directory, with no global config and no pre-commit variables set.
func newRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("PRE_COMMIT_TO_REF", "")
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

// recordedCommit commits a new file with a trailer, as the commit-msg hook does.
func recordedCommit(t *testing.T, name, subject string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(subject), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "add", name)
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte(subject+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := commitverify.Record(path); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "commit", "-q", "-F", path)
}

// plainCommit commits a new file with no trailer, as --no-verify does.
func plainCommit(t *testing.T, name, subject string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(subject), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "add", name)
	gitRun(t, "commit", "-q", "-m", subject)
}

func TestRunAllVerifiedPasses(t *testing.T) {
	newRepo(t)
	recordedCommit(t, "a.txt", "feat: a")
	t.Setenv("PRE_COMMIT_TO_REF", "HEAD")
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code %d, stderr %q; want 0 and no output", code, stderr.String())
	}
}

func TestRunBlocksUnverifiedCommit(t *testing.T) {
	newRepo(t)
	recordedCommit(t, "a.txt", "feat: a")
	plainCommit(t, "b.txt", "feat: b")
	hash := gitRun(t, "rev-parse", "--short=12", "HEAD")
	t.Setenv("PRE_COMMIT_TO_REF", "HEAD")
	var stderr bytes.Buffer

	code := run(nil, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	for _, want := range []string{"not verified at commit time", hash + " feat: b", "git commit --amend --no-edit"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr %q, want it to contain %q", stderr.String(), want)
		}
	}
}

func TestRunChecksHEADWhenNoRefGiven(t *testing.T) {
	newRepo(t)
	plainCommit(t, "a.txt", "feat: a")
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 1 {
		t.Fatalf("code %d, want 1 for an unverified HEAD", code)
	}
}

func TestRunReportsGitFailure(t *testing.T) {
	newRepo(t)
	plainCommit(t, "a.txt", "feat: a")
	t.Setenv("PRE_COMMIT_TO_REF", "no-such-ref")
	var stderr bytes.Buffer

	code := run(nil, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "verify-commits: ") {
		t.Fatalf("code %d, stderr %q; want 1 and a verify-commits error", code, stderr.String())
	}
}
