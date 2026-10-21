// Command require-timeouts fails when a GitHub Actions workflow job has no
// timeout-minutes. Without one, GitHub lets a hung job run for 360 minutes
// (6 hours) before it cancels it.
//
// With --steps, every step must set timeout-minutes too, and a step's literal
// timeout must be below its job's: the job is cancelled first, so a step
// timeout at or above it can never fire.
//
// Jobs that call a reusable workflow (jobs.<id>.uses) are skipped: GitHub does
// not accept timeout-minutes there, so the called workflow's jobs carry it. A
// job or step whose first line ends in `# require-timeouts: allow` is exempt; on
// a job, that also exempts its steps.
package main

import "os"

// main only wires the process to run, which holds the logic and the tests.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
