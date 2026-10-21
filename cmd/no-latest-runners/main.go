// Command no-latest-runners fails when a GitHub Actions workflow runs a job on a
// floating runner label such as ubuntu-latest. GitHub repoints those labels to
// new images (ubuntu-latest moved to Ubuntu 26 from 2026-10-19), so a job's OS
// would change without a commit.
//
// It reads jobs.<id>.runs-on in every form: a label, a list of labels, or a
// {group, labels} object. Labels set through an expression (${{ ... }}) can't be
// resolved and are skipped. A label whose line ends in
// `# no-latest-runners: allow` is exempt.
package main

import "os"

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
