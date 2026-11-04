package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zendern/pre-commit-hooks/internal/yamlnode"
)

const allowMarker = "require-arm-runners: allow"

// finding is one x64 label: where it is and what it is.
type finding struct {
	line  int
	job   string
	label string
}

// run checks each workflow file in args and returns the exit code: 0 when no
// job needs an x64 runner, 1 when one does or a file can't be read.
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
	_, _ = fmt.Fprintln(stderr, "Use an arm64 runner label (e.g. ubuntu-24.04-arm), or put `# require-arm-runners: allow` on the runs-on line of a job that must stay on x64.")
	return 1
}

func checkFile(path string) ([]finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return check(bytes.NewReader(data))
}

// check returns every x64 runner label in one workflow file.
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
		ls := labels(runsOn, at)
		if slices.ContainsFunc(ls, isSelfHosted) {
			continue
		}
		for _, l := range ls {
			if isX64(l.node.Value) && !allowed(l.node) && !allowed(l.at) {
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

func isSelfHosted(l label) bool {
	return l.node.Value == "self-hosted"
}

// isX64 reports whether a GitHub-hosted runner label means an x64 machine. macOS
// labels are arm64 except for -intel, -large (but not -xlarge) and macos-13.
// Other labels are custom runners this hook can't judge.
func isX64(v string) bool {
	if strings.Contains(v, "${{") {
		return false
	}
	if strings.HasPrefix(v, "macos-") {
		return v == "macos-13" || strings.HasSuffix(v, "-intel") || strings.HasSuffix(v, "-large")
	}
	hosted := strings.HasPrefix(v, "ubuntu-") || strings.HasPrefix(v, "windows-")
	return hosted && !strings.HasSuffix(v, "-arm")
}
