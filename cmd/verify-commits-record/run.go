package main

import (
	"fmt"
	"io"
	"os"

	"github.com/zendern/pre-commit-hooks/internal/commitverify"
)

// run records the commit message in args[0] as verified, and returns the exit
// code: 0 when recorded (or deliberately not, see SKIP), 1 when git fails, 2 when
// the message file is missing. It runs on commit-msg, which git reaches only
// after the pre-commit hooks passed, so a commit that fails them is never
// recorded.
//
// A non-empty SKIP means some hook didn't run, so the commit isn't recorded:
// the push gate then holds it back.
func run(args []string, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "verify-commits-record: want one argument, the commit message file")
		return 2
	}
	if skip := os.Getenv("SKIP"); skip != "" {
		_, _ = fmt.Fprintf(stderr, "verify-commits-record: SKIP=%s is set, so this commit is not recorded as verified\n", skip)
		return 0
	}
	if err := commitverify.Record(args[0]); err != nil {
		_, _ = fmt.Fprintf(stderr, "verify-commits-record: %v\n", err)
		return 1
	}
	return 0
}
