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

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zendern/pre-commit-hooks/internal/yamlnode"
)

const allowMarker = "require-timeouts: allow"

// finding is one job or step with no timeout (step is empty for a job), or,
// when stepTimeout is set, a step whose timeout is not below its job's.
type finding struct {
	line        int
	job         string
	step        string
	stepTimeout float64
	jobTimeout  float64
}

func main() {
	steps := flag.Bool("steps", false, "also require timeout-minutes on every step")
	flag.Parse()

	failed := false
	for _, path := range flag.Args() {
		findings, err := checkFile(path, *steps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		for _, f := range findings {
			switch {
			case f.stepTimeout != 0:
				fmt.Printf("%s:%d: step %s timeout-minutes %g is not below job %q timeout-minutes %g\n",
					path, f.line, f.step, f.stepTimeout, f.job, f.jobTimeout)
			case f.step == "":
				fmt.Printf("%s:%d: job %q has no timeout-minutes\n", path, f.line, f.job)
			default:
				fmt.Printf("%s:%d: step %s in job %q has no timeout-minutes\n", path, f.line, f.step, f.job)
			}
			failed = true
		}
	}
	if failed {
		fmt.Fprintln(os.Stderr, "Set timeout-minutes so a hung run is cancelled before GitHub's 6-hour default.")
		if *steps {
			fmt.Fprintln(os.Stderr, "Keep each step's timeout-minutes below its job's, or the job is cancelled first.")
		}
		os.Exit(1)
	}
}

func checkFile(path string, steps bool) ([]finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return check(bytes.NewReader(data), steps)
}

// check returns every job, and with steps every step, that has no timeout in
// one workflow file.
func check(r io.Reader, steps bool) ([]finding, error) {
	var doc yaml.Node
	if err := yaml.NewDecoder(r).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}

	var findings []finding
	_, jobs := yamlnode.Entry(doc.Content[0], "jobs")
	for name, job := range yamlnode.Pairs(jobs) {
		if job.Kind != yaml.MappingNode || allowed(name) || allowed(job) {
			continue
		}
		if _, uses := yamlnode.Entry(job, "uses"); uses != nil {
			continue
		}
		if !hasTimeout(job) {
			findings = append(findings, finding{line: name.Line, job: name.Value})
		}
		if !steps {
			continue
		}
		_, list := yamlnode.Entry(job, "steps")
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		jobTimeout, jobLiteral := literalTimeout(job)
		for i, step := range list.Content {
			if step.Kind != yaml.MappingNode || allowedStep(step) {
				continue
			}
			if !hasTimeout(step) {
				findings = append(findings, finding{line: step.Line, job: name.Value, step: stepLabel(step, i)})
				continue
			}
			if stepTimeout, ok := literalTimeout(step); ok && jobLiteral && jobTimeout > 0 && stepTimeout >= jobTimeout {
				findings = append(findings, finding{
					line: step.Line, job: name.Value, step: stepLabel(step, i),
					stepTimeout: stepTimeout, jobTimeout: jobTimeout,
				})
			}
		}
	}
	return findings, nil
}

// hasTimeout reports whether a job or step sets timeout-minutes to something
// other than null.
func hasTimeout(m *yaml.Node) bool {
	_, v := yamlnode.Entry(m, "timeout-minutes")
	return v != nil && v.Tag != "!!null"
}

// literalTimeout returns a job's or step's timeout-minutes when it is a plain
// number, not an expression.
func literalTimeout(m *yaml.Node) (float64, bool) {
	_, v := yamlnode.Entry(m, "timeout-minutes")
	if v == nil || (v.Tag != "!!int" && v.Tag != "!!float") {
		return 0, false
	}
	n, err := strconv.ParseFloat(v.Value, 64)
	return n, err == nil
}

// allowedStep looks for the marker on a step's first line, where YAML attaches
// it to the step, its first key, or that key's value.
func allowedStep(step *yaml.Node) bool {
	if allowed(step) {
		return true
	}
	for k, v := range yamlnode.Pairs(step) {
		return allowed(k) || allowed(v)
	}
	return false
}

func allowed(n *yaml.Node) bool {
	return n != nil && strings.Contains(n.LineComment, allowMarker)
}

// stepLabel names a step for output: its name, id or uses, else its 1-based
// position in the job.
func stepLabel(step *yaml.Node, i int) string {
	for _, key := range []string{"name", "id", "uses"} {
		if _, v := yamlnode.Entry(step, key); v != nil && v.Value != "" {
			return fmt.Sprintf("%q", v.Value)
		}
	}
	return fmt.Sprintf("#%d", i+1)
}
