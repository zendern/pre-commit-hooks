package commitverify

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo makes an empty repository in a temporary directory and makes it the
// working directory. The global and system git config are ignored, so a hook
// path or signing setting on the machine can't leak into a test.
func newRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(t.TempDir())
	gitRun(t, "init", "-q")
	gitRun(t, "config", "user.name", "Test")
	gitRun(t, "config", "user.email", "test@example.com")
	gitRun(t, "config", "commit.gpgsign", "false")
}

// gitRun runs git in the working directory and fails the test on error.
func gitRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile writes content to name in the working directory and stages it.
func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "add", name)
}

// messageFile writes msg to a fresh file and returns its path.
func messageFile(t *testing.T, msg string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(path, []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// recordedCommit commits a new file the way the commit-msg hook does: the
// trailer is written for the index tree before the commit is made.
func recordedCommit(t *testing.T, name, subject string) {
	t.Helper()
	writeFile(t, name, subject)
	path := messageFile(t, subject+"\n")
	if err := Record(path); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "commit", "-q", "-F", path)
}

// plainCommit commits a new file with no trailer, as --no-verify does.
func plainCommit(t *testing.T, name, subject string) {
	t.Helper()
	writeFile(t, name, subject)
	gitRun(t, "commit", "-q", "-m", subject)
}

func subjects(commits []Commit) []string {
	var out []string
	for _, c := range commits {
		out = append(out, c.Subject)
	}
	return out
}

func TestRecordAddsTrailerNamingIndexTree(t *testing.T) {
	newRepo(t)
	writeFile(t, "a.txt", "one")
	tree := gitRun(t, "write-tree")
	path := messageFile(t, "feat: a\n")

	if err := Record(path); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "feat: a\n\nVerified-tree: " + tree + "\n"; string(got) != want {
		t.Errorf("message %q, want %q", got, want)
	}
}

func TestRecordReplacesStaleTrailer(t *testing.T) {
	newRepo(t)
	writeFile(t, "a.txt", "one")
	tree := gitRun(t, "write-tree")
	path := messageFile(t, "feat: a\n\nVerified-tree: "+strings.Repeat("0", 40)+"\n")

	if err := Record(path); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "feat: a\n\nVerified-tree: " + tree + "\n"; string(got) != want {
		t.Errorf("message %q, want %q", got, want)
	}
}

func TestRecordOutsideRepositoryFails(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(t.TempDir())

	if err := Record(messageFile(t, "feat: a\n")); err == nil {
		t.Fatal("Record outside a repository succeeded, want an error")
	}
}

func TestUnverifiedEmptyWhenEveryCommitRecorded(t *testing.T) {
	newRepo(t)
	recordedCommit(t, "a.txt", "feat: a")
	recordedCommit(t, "b.txt", "feat: b")

	got, err := Unverified("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want none", subjects(got))
	}
}

func TestUnverifiedListsUnrecordedCommitsOldestFirst(t *testing.T) {
	newRepo(t)
	recordedCommit(t, "a.txt", "feat: a")
	plainCommit(t, "b.txt", "feat: b")
	recordedCommit(t, "c.txt", "feat: c")
	plainCommit(t, "d.txt", "fix: d")

	got, err := Unverified("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"feat: b", "fix: d"}; strings.Join(subjects(got), "|") != strings.Join(want, "|") {
		t.Errorf("got %v, want %v", subjects(got), want)
	}
}

func TestUnverifiedRejectsTrailerNamingAnotherTree(t *testing.T) {
	newRepo(t)
	writeFile(t, "a.txt", "feat: a")
	path := messageFile(t, "feat: a\n\nVerified-tree: "+strings.Repeat("0", 40)+"\n")
	gitRun(t, "commit", "-q", "-F", path)

	got, err := Unverified("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"feat: a"}; strings.Join(subjects(got), "|") != strings.Join(want, "|") {
		t.Errorf("got %v, want %v", subjects(got), want)
	}
}

func TestUnverifiedSkipsCommitsAlreadyOnRemote(t *testing.T) {
	newRepo(t)
	plainCommit(t, "a.txt", "feat: a")
	gitRun(t, "update-ref", "refs/remotes/origin/main", "HEAD")
	plainCommit(t, "b.txt", "feat: b")

	got, err := Unverified("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"feat: b"}; strings.Join(subjects(got), "|") != strings.Join(want, "|") {
		t.Errorf("got %v, want %v", subjects(got), want)
	}
}

func TestUnverifiedCommitHashAndSubject(t *testing.T) {
	newRepo(t)
	plainCommit(t, "a.txt", "feat: a")
	hash := gitRun(t, "rev-parse", "HEAD")

	got, err := Unverified("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Hash != hash || got[0].Subject != "feat: a" {
		t.Errorf("got %+v, want hash %s subject %q", got, hash, "feat: a")
	}
}

func TestUnverifiedUnknownRevisionFails(t *testing.T) {
	newRepo(t)
	plainCommit(t, "a.txt", "feat: a")

	if _, err := Unverified("no-such-rev"); err == nil {
		t.Fatal("Unverified of an unknown revision succeeded, want an error")
	}
}

func TestUnverifiedOutsideRepositoryFails(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(t.TempDir())

	if _, err := Unverified("HEAD"); err == nil {
		t.Fatal("Unverified outside a repository succeeded, want an error")
	}
}

// failGit makes every git call whose arguments contain verb fail, and the rest
// run as usual, until the test ends.
func failGit(t *testing.T, verb string) {
	t.Helper()
	real := gitExec
	gitExec = func(args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), verb) {
			return "", errors.New("injected failure")
		}
		return real(args...)
	}
	t.Cleanup(func() { gitExec = real })
}

func TestUnverifiedNothingToPush(t *testing.T) {
	newRepo(t)
	plainCommit(t, "a.txt", "feat: a")
	gitRun(t, "update-ref", "refs/remotes/origin/main", "HEAD")

	got, err := Unverified("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want none", subjects(got))
	}
}

func TestRecordReportsInterpretTrailersFailure(t *testing.T) {
	newRepo(t)
	writeFile(t, "a.txt", "one")
	failGit(t, "interpret-trailers")

	if err := Record(messageFile(t, "feat: a\n")); err == nil {
		t.Fatal("Record succeeded with interpret-trailers failing, want an error")
	}
}

func TestUnverifiedReportsFailureAtEachGitCall(t *testing.T) {
	for _, verb := range []string{"rev-parse", "trailers", "%s"} {
		t.Run(verb, func(t *testing.T) {
			newRepo(t)
			plainCommit(t, "a.txt", "feat: a")
			failGit(t, verb)

			if _, err := Unverified("HEAD"); err == nil {
				t.Fatalf("Unverified succeeded with %q failing, want an error", verb)
			}
		})
	}
}
