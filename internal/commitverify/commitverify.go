// Package commitverify finds the commits a push would send that are not
// signed, or whose signature is bad.
package commitverify

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Status is a commit's signature status, as git reports it in %G?.
type Status string

// The statuses git reports in %G?.
const (
	Good       Status = "G" // good signature
	Untrusted  Status = "U" // good signature by a key not trusted (GPG only)
	Bad        Status = "B" // bad signature
	Unchecked  Status = "E" // signature git can't check
	Expired    Status = "X" // good signature that has expired
	ExpiredKey Status = "Y" // good signature by an expired key
	RevokedKey Status = "R" // good signature by a revoked key
	None       Status = "N" // no signature
)

// Reason describes a status in a few words, for a failure message.
func (s Status) Reason() string {
	switch s {
	case Good:
		return "good signature"
	case Untrusted:
		return "key not trusted"
	case Bad:
		return "bad signature"
	case Unchecked:
		return "signature git can't check"
	case Expired:
		return "expired signature"
	case ExpiredKey:
		return "signature by an expired key"
	case RevokedKey:
		return "signature by a revoked key"
	case None:
		return "no signature"
	}
	return "signature status " + string(s)
}

// Commit is a commit that a push would send, with its signature status.
type Commit struct {
	Hash    string
	Parent  string // first parent, empty for a root commit
	Status  Status
	Subject string
}

// Unsigned returns the commits reachable from rev that no remote-tracking ref
// has, and that fail the signature check, oldest first.
//
// By default a commit fails with no signature (None) or a bad one (Bad). Any
// other status passes, Unchecked included: the commit is signed, and failing it
// would block everyone without a local trust setup. With strict set, only Good
// passes: Untrusted means the key is in the keyring, not that it is trusted.
//
// git reports an SSH signature as N when gpg.ssh.allowedSignersFile is unset,
// the same as no signature, so a None commit whose raw object carries a
// signature header is treated as Unchecked.
func Unsigned(rev string, strict bool) ([]Commit, error) {
	out, err := gitExec("log", "--reverse", "--format=%H%x00%P%x00%G?%x00%s", rev, "--not", "--remotes")
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil // nothing to push
	}

	var unsigned []Commit
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, "\x00", 4)
		if len(fields) < 4 {
			return nil, fmt.Errorf("unexpected git log line %q", line)
		}
		hash, status := fields[0], Status(fields[2])
		parent, _, _ := strings.Cut(fields[1], " ")
		if status == None {
			signed, err := hasSignature(hash)
			if err != nil {
				return nil, err
			}
			if signed {
				status = Unchecked
			}
		}
		if status.fails(strict) {
			unsigned = append(unsigned, Commit{Hash: hash, Parent: parent, Status: status, Subject: fields[3]})
		}
	}
	return unsigned, nil
}

// fails reports whether the status fails the check.
func (s Status) fails(strict bool) bool {
	if strict {
		return s != Good
	}
	return s == None || s == Bad
}

// hasSignature reports whether the commit object has a signature header. The
// headers end at the first blank line, where the message starts.
func hasSignature(hash string) (bool, error) {
	raw, err := gitExec("cat-file", "commit", hash)
	if err != nil {
		return false, err
	}
	headers, _, _ := strings.Cut(raw, "\n\n")
	for _, line := range strings.Split(headers, "\n") {
		if strings.HasPrefix(line, "gpgsig ") || strings.HasPrefix(line, "gpgsig-sha256 ") {
			return true, nil
		}
	}
	return false, nil
}

// gitExec runs git; tests replace it to make one call fail.
var gitExec = git

// git runs git with args in the working directory and returns its trimmed
// standard output. A failure carries git's own error text.
func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
