// Command verify-commits-record is a commit-msg hook: it records that a commit
// passed the pre-commit hooks, by adding a Verified-tree trailer to the message.
// verify-commits checks that trailer when the commit is pushed.
package main

import (
	"os"
)

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
