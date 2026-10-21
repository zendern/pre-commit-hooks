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

// run checks each workflow file in args and returns the exit code: 0 when
// every runner label is pinned, 1 when one floats or a file can't be read.
func run(args []string, stdout, stderr io.Writer) int {
	failed := false
	for _, path := range args {
		findings, err := checkFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		for _, f := range findings {
			_, _ = fmt.Fprintf(stdout, "%s:%d: job %q runs on %q\n", path, f.line, f.job, f.label)
			failed = true
		}
	}
	if !failed {
		return 0
	}
	_, _ = fmt.Fprintln(stderr, "Pin runner labels to a version (e.g. ubuntu-24.04) instead of *-latest.")
	return 1
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

	var findings []finding
	_, jobs := yamlnode.Entry(yamlnode.Root(&doc), "jobs")
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
