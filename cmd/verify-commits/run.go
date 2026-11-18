package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/zendern/pre-commit-hooks/internal/commitverify"
)

// run blocks a push that sends a commit failing the signature check, and
// returns the exit code: 0 when every commit passes, 1 when one fails or git
// fails, 2 for a bad flag. By default a commit fails with no signature or a bad
// one; --strict also fails one git can't check, or that is expired or revoked.
// It checks the commits pre-commit says are being pushed (PRE_COMMIT_TO_REF),
// or HEAD when that isn't set.
func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("verify-commits", flag.ContinueOnError)
	flags.SetOutput(stderr)
	strict := flags.Bool("strict", false, "fail every commit without a good signature, including ones git can't check")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	rev := os.Getenv("PRE_COMMIT_TO_REF")
	if rev == "" {
		rev = "HEAD"
	}

	unsigned, err := commitverify.Unsigned(rev, *strict)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "verify-commits: %v\n", err)
		return 1
	}
	if len(unsigned) == 0 {
		return 0
	}

	_, _ = fmt.Fprintln(stderr, "Push blocked: these commits are not signed with a signature this hook accepts:")
	for _, c := range unsigned {
		_, _ = fmt.Fprintf(stderr, "  %s %s (%s)\n", c.Hash[:min(len(c.Hash), 12)], c.Subject, c.Status.Reason())
	}
	_, _ = fmt.Fprintln(stderr, "Sign them: git commit --amend --no-edit -S, or for several: git rebase --exec 'git commit --amend --no-edit -S' <last pushed commit>")
	return 1
}
