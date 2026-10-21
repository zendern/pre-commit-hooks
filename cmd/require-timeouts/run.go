package main

import (
	"bytes"
	"cmp"
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

// run checks each workflow file in args and returns the exit code: 0 when every
// job (and with --steps every step) has a timeout, 1 when one does not or a
// file can't be read, 2 for a bad flag.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("require-timeouts", flag.ContinueOnError)
	flags.SetOutput(stderr)
	steps := flags.Bool("steps", false, "also require timeout-minutes on every step")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	failed := false
	for _, path := range flags.Args() {
		findings, err := checkFile(path, *steps)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		for _, f := range findings {
			_, _ = fmt.Fprintln(stdout, describe(path, f))
			failed = true
		}
	}
	if !failed {
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "Set timeout-minutes so a hung run is cancelled before GitHub's 6-hour default.")
	if *steps {
		_, _ = fmt.Fprintln(stderr, "Keep each step's timeout-minutes below its job's, or the job is cancelled first.")
	}
	return 1
}

// describe is the one-line report for a finding.
func describe(path string, f finding) string {
	if f.stepTimeout != 0 {
		return fmt.Sprintf("%s:%d: step %s timeout-minutes %g is not below job %q timeout-minutes %g",
			path, f.line, f.step, f.stepTimeout, f.job, f.jobTimeout)
	}
	if f.step == "" {
		return fmt.Sprintf("%s:%d: job %q has no timeout-minutes", path, f.line, f.job)
	}
	return fmt.Sprintf("%s:%d: step %s in job %q has no timeout-minutes", path, f.line, f.step, f.job)
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

	var findings []finding
	_, jobs := yamlnode.Entry(yamlnode.Root(&doc), "jobs")
	for name, job := range yamlnode.Pairs(jobs) {
		// Aliases are followed, and a finding is reported where the value is
		// used: the job's key for a job reused whole (`b: *a`), else the
		// outermost alias passed through. An allow marker on the anchored job
		// or step covers every use, one on an alias that use only.
		if yamlnode.Resolve(job).Kind != yaml.MappingNode || allowed(name) || allowed(job) {
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
		var at *yaml.Node
		if job.Kind == yaml.AliasNode {
			at = name
		}
		_, list := yamlnode.Entry(job, "steps")
		if list != nil && list.Kind == yaml.AliasNode {
			at = cmp.Or(at, list)
		}
		list = yamlnode.Resolve(list)
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		jobTimeout, jobLiteral := literalTimeout(job)
		for i, item := range list.Content {
			step := yamlnode.Resolve(item)
			if step.Kind != yaml.MappingNode || allowed(item) || allowedStep(step) {
				continue
			}
			line := cmp.Or(at, item).Line
			if !hasTimeout(step) {
				findings = append(findings, finding{line: line, job: name.Value, step: stepLabel(step, i)})
				continue
			}
			if stepTimeout, ok := literalTimeout(step); ok && jobLiteral && jobTimeout > 0 && stepTimeout >= jobTimeout {
				findings = append(findings, finding{
					line: line, job: name.Value, step: stepLabel(step, i),
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
	v = yamlnode.Resolve(v)
	return v != nil && v.Tag != "!!null"
}

// literalTimeout returns a job's or step's timeout-minutes when it is a plain
// number, not an expression.
func literalTimeout(m *yaml.Node) (float64, bool) {
	_, v := yamlnode.Entry(m, "timeout-minutes")
	v = yamlnode.Resolve(v)
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
