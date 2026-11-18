package commitverify

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo makes an empty repository in a temporary directory, makes it the
// working directory, and gives it an SSH signing key. The global and system git
// config are ignored, so the machine's own signing setup can't leak in. It
// returns the public key line, for an allowed-signers file.
func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	keyDir := t.TempDir()
	key := filepath.Join(keyDir, "signer")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "test", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	gitRun(t, "init", "-q")
	gitRun(t, "config", "user.name", "Test")
	gitRun(t, "config", "user.email", "test@example.com")
	gitRun(t, "config", "commit.gpgsign", "false")
	gitRun(t, "config", "gpg.format", "ssh")
	gitRun(t, "config", "user.signingkey", key+".pub")
	return strings.TrimSpace(string(pub))
}

// trustKey lets git check signatures made with pub, so they report G.
func trustKey(t *testing.T, pub string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "allowed_signers")
	if err := os.WriteFile(path, []byte("test@example.com "+pub+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "config", "gpg.ssh.allowedSignersFile", path)
}

func gitRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit commits a new file, signed when signed is set.
func commit(t *testing.T, name, subject string, signed bool) {
	t.Helper()
	if err := os.WriteFile(name, []byte(subject), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "add", name)
	args := []string{"commit", "-q", "-m", subject}
	if signed {
		args = append(args, "-S")
	}
	gitRun(t, args...)
}

func subjects(commits []Commit) string {
	var out []string
	for _, c := range commits {
		out = append(out, string(c.Status)+" "+c.Subject)
	}
	return strings.Join(out, "|")
}

func TestUnsignedEmptyWhenEveryCommitSigned(t *testing.T) {
	trustKey(t, newRepo(t))
	commit(t, "a.txt", "feat: a", true)
	commit(t, "b.txt", "feat: b", true)

	got, err := Unsigned("HEAD", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %q, want none", subjects(got))
	}
}

func TestUnsignedListsUnsignedCommitsOldestFirst(t *testing.T) {
	trustKey(t, newRepo(t))
	commit(t, "a.txt", "feat: a", true)
	commit(t, "b.txt", "feat: b", false)
	commit(t, "c.txt", "feat: c", true)
	commit(t, "d.txt", "fix: d", false)

	got, err := Unsigned("HEAD", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "N feat: b|N fix: d"; subjects(got) != want {
		t.Errorf("got %q, want %q", subjects(got), want)
	}
}

func TestUnsignedPassesSignatureGitCannotCheck(t *testing.T) {
	newRepo(t) // no allowed-signers file, so the signature reports E
	commit(t, "a.txt", "feat: a", true)

	got, err := Unsigned("HEAD", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %q, want none", subjects(got))
	}
}

func TestUnsignedSkipsCommitsAlreadyOnRemote(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	gitRun(t, "update-ref", "refs/remotes/origin/main", "HEAD")
	commit(t, "b.txt", "feat: b", false)

	got, err := Unsigned("HEAD", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "N feat: b"; subjects(got) != want {
		t.Errorf("got %q, want %q", subjects(got), want)
	}
}

func TestUnsignedNothingToPush(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	gitRun(t, "update-ref", "refs/remotes/origin/main", "HEAD")

	got, err := Unsigned("HEAD", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %q, want none", subjects(got))
	}
}

func TestUnsignedCommitHash(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	hash := gitRun(t, "rev-parse", "HEAD")

	got, err := Unsigned("HEAD", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Hash != hash {
		t.Errorf("got %+v, want hash %s", got, hash)
	}
}

func TestUnsignedUnknownRevisionFails(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)

	if _, err := Unsigned("no-such-rev", false); err == nil {
		t.Fatal("Unsigned of an unknown revision succeeded, want an error")
	}
}

func TestUnsignedReportsCatFileFailure(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	gitExec = func(args ...string) (string, error) {
		if args[0] == "cat-file" {
			return "", errors.New("injected failure")
		}
		return git(args...)
	}
	t.Cleanup(func() { gitExec = git })

	if _, err := Unsigned("HEAD", false); err == nil {
		t.Fatal("Unsigned succeeded with cat-file failing, want an error")
	}
}

func TestUnsignedStrictFailsSignatureGitCannotCheck(t *testing.T) {
	newRepo(t) // no allowed-signers file: git can't check the SSH signature
	commit(t, "a.txt", "feat: a", true)

	got, err := Unsigned("HEAD", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "E feat: a"; subjects(got) != want {
		t.Errorf("got %q, want %q", subjects(got), want)
	}
}

func TestUnsignedStrictPassesGoodSignature(t *testing.T) {
	trustKey(t, newRepo(t))
	commit(t, "a.txt", "feat: a", true)
	commit(t, "b.txt", "feat: b", false)

	got, err := Unsigned("HEAD", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "N feat: b"; subjects(got) != want {
		t.Errorf("got %q, want %q", subjects(got), want)
	}
}

func TestFailsByStatus(t *testing.T) {
	for _, tc := range []struct {
		status      Status
		def, strict bool
	}{
		{Good, false, false},
		{Untrusted, false, true},
		{None, true, true},
		{Bad, true, true},
		{Unchecked, false, true},
		{Expired, false, true},
		{ExpiredKey, false, true},
		{RevokedKey, false, true},
	} {
		if got := tc.status.fails(false); got != tc.def {
			t.Errorf("%q.fails(false) = %v, want %v", tc.status, got, tc.def)
		}
		if got := tc.status.fails(true); got != tc.strict {
			t.Errorf("%q.fails(true) = %v, want %v", tc.status, got, tc.strict)
		}
	}
}

func TestReason(t *testing.T) {
	for status, want := range map[Status]string{
		Good:       "good signature",
		Untrusted:  "key not trusted",
		Bad:        "bad signature",
		Unchecked:  "signature git can't check",
		Expired:    "expired signature",
		ExpiredKey: "signature by an expired key",
		RevokedKey: "signature by a revoked key",
		None:       "no signature",
		"Z":        "signature status Z",
	} {
		if got := status.Reason(); got != want {
			t.Errorf("%q.Reason() = %q, want %q", status, got, want)
		}
	}
}
