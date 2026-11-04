// Command require-arm-runners fails when a GitHub Actions workflow runs a job on
// an x64 GitHub-hosted runner label such as ubuntu-24.04. A standard arm64 Linux
// runner costs less per minute in a private repository, so included usage lasts
// longer.
//
// It reads jobs.<id>.runs-on in every form: a label, a list of labels, or a
// {group, labels} object. Only GitHub's own label families are judged, so custom
// labels pass, and a job with the self-hosted label is skipped whole. Labels set
// through an expression (${{ ... }}) can't be resolved and are skipped. A label
// whose line ends in `# require-arm-runners: allow` is exempt.
package main

import "os"

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
