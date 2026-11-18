// Command verify-commits is a pre-push hook: it fails a push that sends a commit
// which didn't pass the commit-time hooks, because it was made with --no-verify.
// Commits pass when verify-commits-record recorded them on commit-msg.
//
// Commits that are reachable from the pushed revision but not from any
// remote-tracking ref are checked, so a commit already on the remote is never
// asked for a record.
package main

import (
	"os"
)

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
