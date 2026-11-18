package main

import (
	"fmt"
	"io"
	"os"

	"github.com/zendern/pre-commit-hooks/internal/commitverify"
)

// run blocks a push that sends a commit without a verification record, and
// returns the exit code: 0 when every commit is recorded, 1 when one isn't or
// git fails. It checks the commits pre-commit says are being pushed
// (PRE_COMMIT_TO_REF), or HEAD when that isn't set. It takes no arguments:
// the hook runs at pre-push, where git's own arguments mean nothing here.
func run(_ []string, stderr io.Writer) int {
	rev := os.Getenv("PRE_COMMIT_TO_REF")
	if rev == "" {
		rev = "HEAD"
	}

	unverified, err := commitverify.Unverified(rev)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify-commits: %v\n", err)
		return 1
	}
	if len(unverified) == 0 {
		return 0
	}

	_, _ = fmt.Fprintln(stderr, "Push blocked: these commits were not verified at commit time (committed with --no-verify, or amended or rebased since).")
	for _, c := range unverified {
		_, _ = fmt.Fprintf(stderr, "  %s %s\n", c.Hash[:min(len(c.Hash), 12)], c.Subject)
	}
	_, _ = fmt.Fprintln(stderr, "Commit them again with the hooks on: git commit --amend --no-edit, or for several: git rebase --exec 'git commit --amend --no-edit' <last pushed commit>")
	return 1
}
