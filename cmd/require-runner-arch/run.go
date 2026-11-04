package main

import (
	"bytes"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/zendern/pre-commit-hooks/internal/yamlnode"
)

const (
	allowMarker = "require-runner-arch: allow"

	arm64 = "arm64"
	x64   = "x64"
)

// policy is what the flags ask for: the architecture every job must run on, and
// which custom labels count as being on it (nil when custom labels can't be
// judged).
type policy struct {
	arch   string
	custom *regexp.Regexp
}

// finding is one label on the wrong architecture, or a job's whole label list
// when none of its labels says what it runs on. With warn set it doesn't fail the
// hook, because the job might still be on the required architecture.
type finding struct {
	line  int
	job   string
	label string
	warn  bool
}

// run checks each workflow file in args and returns the exit code: 0 when every
// job is on the required architecture or can't be judged, 1 when one is not or
// a file can't be read, 2 for a bad flag.
func run(args []string, stdout, stderr io.Writer) int {
	var p policy
	var matcher string
	flags := flag.NewFlagSet("require-runner-arch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Func("arch", "the required architecture: arm64 or x64", func(v string) error {
		if v != arm64 && v != x64 {
			return errors.New("must be arm64 or x64")
		}
		p.arch = v
		return nil
	})
	flags.Func("custom-label-matcher", "regular expression for custom labels on the required architecture", func(v string) (err error) {
		matcher = v
		// GitHub's labels are case-insensitive.
		p.custom, err = regexp.Compile("(?i)" + v)
		return err
	})
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if p.arch == "" {
		_, _ = fmt.Fprintln(stderr, "--arch is required: arm64 or x64")
		return 2
	}

	failed := false
	for _, path := range flags.Args() {
		findings, err := checkFile(path, p)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		for _, f := range findings {
			if f.warn {
				_, _ = fmt.Fprintf(stdout, "%s:%d: warning: job %q runs on %q, architecture unknown\n", path, f.line, f.job, f.label)
				continue
			}
			_, _ = fmt.Fprintf(stdout, "%s:%d: job %q runs on %q\n", path, f.line, f.job, f.label)
			failed = true
		}
	}
	if !failed {
		return 0
	}
	if p.arch == arm64 {
		_, _ = fmt.Fprintln(stderr, "Use an arm64 runner label (e.g. ubuntu-24.04-arm), or put `# require-runner-arch: allow` on the runs-on line of a job that has to stay on another architecture.")
	} else {
		_, _ = fmt.Fprintln(stderr, "Use an x64 runner label (e.g. ubuntu-24.04), or put `# require-runner-arch: allow` on the runs-on line of a job that has to stay on another architecture.")
	}
	if p.custom != nil {
		_, _ = fmt.Fprintf(stderr, "A custom runner counts as %s only when one of its labels matches --custom-label-matcher %q.\n", p.arch, matcher)
	}
	return 1
}

func checkFile(path string, p policy) ([]finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return check(bytes.NewReader(data), p)
}

// check returns every job on the wrong architecture in one workflow file.
func check(r io.Reader, p policy) ([]finding, error) {
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
		findings = append(findings, judge(name.Value, labels(runsOn, at), p)...)
	}
	return findings, nil
}

// judge returns what is wrong with one job's labels. A custom match settles the
// job whatever else it lists, since a custom runner can carry any label.
func judge(job string, ls []label, p policy) []finding {
	if len(ls) == 0 {
		return nil
	}
	if p.custom != nil && slices.ContainsFunc(ls, func(l label) bool { return p.custom.MatchString(l.node.Value) }) {
		return nil
	}
	if p.custom == nil && slices.ContainsFunc(ls, isSelfHosted) {
		if slices.ContainsFunc(ls, labelAllowed) {
			return nil
		}
		return []finding{wholeJob(job, ls, true)}
	}

	var findings []finding
	for _, l := range ls {
		arch := archOf(l.node.Value)
		if arch != "" && arch != p.arch && !labelAllowed(l) {
			findings = append(findings, finding{line: l.at.Line, job: job, label: l.node.Value})
		}
	}
	if slices.ContainsFunc(ls, isSettled) {
		return findings
	}
	// Nothing says what the job runs on. That only fails when the matcher could
	// have said so: an expression might still resolve to a match.
	warn := p.custom == nil || slices.ContainsFunc(ls, func(l label) bool { return isExpression(l.node.Value) })
	return []finding{wholeJob(job, ls, warn)}
}

// wholeJob reports a job once, with every label, rather than label by label.
func wholeJob(job string, ls []label, warn bool) finding {
	values := make([]string, 0, len(ls))
	for _, l := range ls {
		values = append(values, l.node.Value)
	}
	return finding{line: ls[0].at.Line, job: job, label: strings.Join(values, ", "), warn: warn}
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

func labelAllowed(l label) bool {
	return allowed(l.node) || allowed(l.at)
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
	return strings.EqualFold(l.node.Value, "self-hosted")
}

// isSettled reports whether a label says all the hook needs about its job: its
// architecture is known, or the job is exempt through it.
func isSettled(l label) bool {
	return archOf(l.node.Value) != "" || labelAllowed(l)
}

func isExpression(v string) bool {
	return strings.Contains(v, "${{")
}

// archOf returns the architecture of a GitHub-hosted runner label, or "" for an
// expression or a custom label. macOS labels are arm64 except for -intel, -large
// (but not -xlarge) and macos-13. Case is ignored because GitHub's labels do.
func archOf(v string) string {
	if isExpression(v) {
		return ""
	}
	v = strings.ToLower(v)
	if strings.HasPrefix(v, "macos-") {
		if v == "macos-13" || strings.HasSuffix(v, "-intel") || strings.HasSuffix(v, "-large") {
			return x64
		}
		return arm64
	}
	if !strings.HasPrefix(v, "ubuntu-") && !strings.HasPrefix(v, "windows-") {
		return ""
	}
	if strings.HasSuffix(v, "-arm") {
		return arm64
	}
	return x64
}
