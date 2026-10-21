package main

import (
	"bytes"
	"cmp"
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
		// A job reused whole (`b: *a`) is reported at its own key.
		var at *yaml.Node
		if job.Kind == yaml.AliasNode {
			at = name
		}
		key, runsOn := yamlnode.Entry(job, "runs-on")
		// The marker can sit on the runs-on line itself (where YAML attaches
		// it to the key or to a flow list), on a block-list item's line, or
		// on a reused job's `b: *a` line (where it attaches to the alias).
		if allowed(job) || allowed(key) || allowed(runsOn) {
			continue
		}
		for _, l := range labels(runsOn, at) {
			if isFloating(l.node) && !allowed(l.node) && !allowed(l.at) {
				findings = append(findings, finding{line: l.at.Line, job: name.Value, label: l.node.Value})
			}
		}
	}
	return findings, nil
}

// label is one runner label, after aliases are followed, and the node a finding
// about it is reported at: the outermost alias it was reached through, a reused
// job's key included, else the label itself. An allow marker on either counts:
// one on the anchored label covers every use, one on an alias that use only.
type label struct {
	node *yaml.Node
	at   *yaml.Node
}

func allowed(n *yaml.Node) bool {
	return n != nil && strings.Contains(n.LineComment, allowMarker)
}

// labels flattens a runs-on value into its labels. at is the outermost alias
// already passed through, or nil.
func labels(runsOn, at *yaml.Node) []label {
	if runsOn == nil {
		return nil
	}
	switch runsOn.Kind {
	case yaml.AliasNode:
		return labels(runsOn.Alias, cmp.Or(at, runsOn))
	case yaml.ScalarNode:
		return []label{{node: runsOn, at: cmp.Or(at, runsOn)}}
	case yaml.SequenceNode:
		var all []label
		for _, item := range runsOn.Content {
			all = append(all, labels(item, at)...)
		}
		return all
	default: // a {group, labels} mapping
		_, l := yamlnode.Entry(runsOn, "labels")
		return labels(l, at)
	}
}

func isFloating(label *yaml.Node) bool {
	return label.Kind == yaml.ScalarNode &&
		strings.HasSuffix(label.Value, "-latest") &&
		!strings.Contains(label.Value, "${{")
}
