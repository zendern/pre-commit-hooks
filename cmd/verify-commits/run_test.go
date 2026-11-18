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
// working directory, with an SSH signing key, no global config and no
// pre-commit variables set.
func newRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("PRE_COMMIT_TO_REF", "")
	key := filepath.Join(t.TempDir(), "signer")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "test", "-f", key).CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	t.Chdir(t.TempDir())
	gitRun(t, "init", "-q")
	gitRun(t, "config", "user.name", "Test")
	gitRun(t, "config", "user.email", "test@example.com")
	gitRun(t, "config", "commit.gpgsign", "false")
	gitRun(t, "config", "gpg.format", "ssh")
	gitRun(t, "config", "user.signingkey", key+".pub")
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

func TestRunAllSignedPasses(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", true)
	t.Setenv("PRE_COMMIT_TO_REF", "HEAD")
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code %d, stderr %q; want 0 and no output", code, stderr.String())
	}
}

func TestRunBlocksUnsignedCommit(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", true)
	commit(t, "b.txt", "feat: b", false)
	hash := gitRun(t, "rev-parse", "--short=12", "HEAD")
	parent := gitRun(t, "rev-parse", "--short=12", "HEAD~1")
	t.Setenv("PRE_COMMIT_TO_REF", "HEAD")
	var stderr bytes.Buffer

	code := run(nil, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	for _, want := range []string{"not signed with a signature", hash + " feat: b (no signature)", "git rebase --rebase-merges --exec 'git commit --amend --no-edit -S' " + parent + "\n"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr %q, want it to contain %q", stderr.String(), want)
		}
	}
	if strings.Contains(stderr.String(), "feat: a") {
		t.Errorf("stderr %q lists the signed commit", stderr.String())
	}
}

func TestRunNamesBadSignature(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", true)
	// Rewrite the signed commit's message without re-signing, so the signature
	// no longer matches its content.
	raw := gitRun(t, "cat-file", "commit", "HEAD")
	forged := strings.Replace(raw, "feat: a", "feat: forged", 1)
	cmd := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(forged + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, "update-ref", "HEAD", strings.TrimSpace(string(out)))
	allowed := filepath.Join(t.TempDir(), "allowed_signers")
	pub, err := os.ReadFile(gitRun(t, "config", "user.signingkey"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(allowed, append([]byte("test@example.com "), pub...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, "config", "gpg.ssh.allowedSignersFile", allowed)
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 1 || !strings.Contains(stderr.String(), "feat: forged (bad signature)") {
		t.Fatalf("code %d, stderr %q; want 1 and a bad signature", code, stderr.String())
	}
}

func TestRunChecksHEADWhenNoRefGiven(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 1 {
		t.Fatalf("code %d, want 1 for an unsigned HEAD", code)
	}
}

func TestRunReportsGitFailure(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	t.Setenv("PRE_COMMIT_TO_REF", "no-such-ref")
	var stderr bytes.Buffer

	code := run(nil, &stderr)

	if code != 1 || !strings.HasPrefix(stderr.String(), "verify-commits: ") {
		t.Fatalf("code %d, stderr %q; want 1 and a verify-commits error", code, stderr.String())
	}
}

func TestRunStrictBlocksSignatureGitCannotCheck(t *testing.T) {
	newRepo(t) // no allowed-signers file
	commit(t, "a.txt", "feat: a", true)
	var stderr bytes.Buffer

	if code := run(nil, &stderr); code != 0 {
		t.Fatalf("default: code %d, stderr %q; want 0", code, stderr.String())
	}
	if code := run([]string{"--strict"}, &stderr); code != 1 || !strings.Contains(stderr.String(), "feat: a (signature git can't check)") {
		t.Fatalf("--strict: code %d, stderr %q; want 1 naming the unchecked signature", code, stderr.String())
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var stderr bytes.Buffer

	if code := run([]string{"--nope"}, &stderr); code != 2 {
		t.Fatalf("code %d, want 2", code)
	}
}

func TestRunResignsFromRootWhenFirstCommitIsUnsigned(t *testing.T) {
	newRepo(t)
	commit(t, "a.txt", "feat: a", false)
	var stderr bytes.Buffer

	run(nil, &stderr)

	if want := "git rebase --rebase-merges --exec 'git commit --amend --no-edit -S' --root\n"; !strings.HasSuffix(stderr.String(), want) {
		t.Errorf("stderr %q, want it to end with %q", stderr.String(), want)
	}
}
