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

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zendern/pre-commit-hooks/internal/yamlnode"
)

const allowMarker = "no-latest-runners: allow"

// finding is one floating label: where it is and what it is.
type finding struct {
	line  int
	job   string
	label string
}

func main() {
	failed := false
	for _, path := range os.Args[1:] {
		findings, err := checkFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		for _, f := range findings {
			fmt.Printf("%s:%d: job %q runs on %q\n", path, f.line, f.job, f.label)
			failed = true
		}
	}
	if failed {
		fmt.Fprintln(os.Stderr, "Pin runner labels to a version (e.g. ubuntu-24.04) instead of *-latest.")
		os.Exit(1)
	}
}

func checkFile(path string) ([]finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return check(bytes.NewReader(data))
}

// check returns every floating runner label in one workflow file.
func check(r io.Reader) ([]finding, error) {
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
		key, runsOn := yamlnode.Entry(job, "runs-on")
		// The marker can sit on the runs-on line itself (where YAML attaches
		// it to the key or to a flow list) or on a block-list item's line.
		if allowed(key) || allowed(runsOn) {
			continue
		}
		for _, label := range labels(runsOn) {
			if isFloating(label) && !allowed(label) {
				findings = append(findings, finding{line: label.Line, job: name.Value, label: label.Value})
			}
		}
	}
	return findings, nil
}

func allowed(n *yaml.Node) bool {
	return n != nil && strings.Contains(n.LineComment, allowMarker)
}

// labels flattens a runs-on value into its label nodes.
func labels(runsOn *yaml.Node) []*yaml.Node {
	if runsOn == nil {
		return nil
	}
	switch runsOn.Kind {
	case yaml.ScalarNode:
		return []*yaml.Node{runsOn}
	case yaml.SequenceNode:
		return runsOn.Content
	case yaml.MappingNode: // {group, labels}
		_, l := yamlnode.Entry(runsOn, "labels")
		return labels(l)
	}
	return nil
}

func isFloating(label *yaml.Node) bool {
	return label.Kind == yaml.ScalarNode &&
		strings.HasSuffix(label.Value, "-latest") &&
		!strings.Contains(label.Value, "${{")
}
