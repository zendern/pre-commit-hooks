// Command verify-commits is a pre-push hook: it fails a push that sends an
// unsigned commit, or one with a bad signature. Committing unsigned stays
// possible; the push is what stops.
//
// Commits that are reachable from the pushed revision but not from any
// remote-tracking ref are checked, so a commit already on the remote is never
// checked again.
package main

import (
	"os"
)

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
