// Package commitverify records that a commit passed its commit-time hooks and
// finds the commits a push would send that lack that record.
//
// The record is a commit-message trailer, so it travels with the commit and
// needs no file. Its value is the tree the commit holds: a trailer copied onto
// a commit with different content, by a rebase for example, doesn't count.
package commitverify

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Trailer is the commit-message trailer that records verification.
const Trailer = "Verified-tree"

// Commit is a commit that a push would send.
type Commit struct {
	Hash    string
	Subject string
}

// gitExec runs git and returns its trimmed standard output. Tests replace it to
// make git fail partway through a walk.
var gitExec = execGit

// Record adds the Trailer to the commit message in path, naming the tree the
// index holds now. It is meant for the commit-msg hook, which git runs only
// after the pre-commit hooks passed and which --no-verify skips.
func Record(path string) error {
	tree, err := gitExec("write-tree")
	if err != nil {
		return err
	}
	_, err = gitExec("interpret-trailers", "--in-place", "--if-exists", "replace", "--trailer", Trailer+": "+tree, path)
	return err
}

// Unverified returns the commits reachable from rev that no remote-tracking ref
// has, and that lack a Trailer naming their own tree. They are oldest first.
func Unverified(rev string) ([]Commit, error) {
	out, err := gitExec("rev-list", "--reverse", rev, "--not", "--remotes")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var unverified []Commit
	for _, hash := range strings.Split(out, "\n") {
		ok, err := recorded(hash)
		if err != nil {
			return nil, err
		}
		if ok {
			continue
		}
		subject, err := gitExec("log", "-1", "--format=%s", hash)
		if err != nil {
			return nil, err
		}
		unverified = append(unverified, Commit{Hash: hash, Subject: subject})
	}
	return unverified, nil
}

// recorded reports whether a Trailer in the commit names the commit's own tree.
func recorded(hash string) (bool, error) {
	tree, err := gitExec("rev-parse", hash+"^{tree}")
	if err != nil {
		return false, err
	}
	values, err := gitExec("log", "-1", "--format=%(trailers:key="+Trailer+",valueonly)", hash)
	if err != nil {
		return false, err
	}
	for _, value := range strings.Split(values, "\n") {
		if strings.TrimSpace(value) == tree {
			return true, nil
		}
	}
	return false, nil
}

// execGit runs git with args in the working directory. A failure carries git's
// own error text.
func execGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
