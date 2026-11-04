// Command require-runner-arch fails when a GitHub Actions workflow runs a job on
// a different CPU architecture from the one --arch requires. A standard arm64
// Linux runner costs less per minute in a private repository, and a project that
// ships amd64 may want its CI on x64.
//
// It reads jobs.<id>.runs-on in every form: a label, a list of labels, or a
// {group, labels} object. Only GitHub's own labels have a known architecture.
// Other labels and jobs with the self-hosted label get a warning and don't fail
// the hook, unless --custom-label-matcher says which custom labels are on the
// required architecture. Labels set through an expression (${{ ... }}) can't be
// resolved, so they get a warning either way. A label whose line ends in
// `# require-runner-arch: allow` is exempt.
package main

import "os"

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
