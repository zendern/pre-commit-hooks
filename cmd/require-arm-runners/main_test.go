package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const hint = "Use an arm64 runner label (e.g. ubuntu-24.04-arm), or put `# require-arm-runners: allow` on the runs-on line of a job that must stay on x64.\n"

func TestRunArm(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{filepath.Join("testdata", "arm.yml")}, &stdout, &stderr)

	if code != 0 || stdout.String() != "" || stderr.String() != "" {
		t.Fatalf("code %d, stdout %q, stderr %q; want 0 and no output", code, stdout.String(), stderr.String())
	}
}

func TestRunNoFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("code %d, want 0", code)
	}
}

func TestRunX64(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join("testdata", "several-jobs.yml")

	code := run([]string{path}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	want := path + `:3: job "a" runs on "ubuntu-24.04"` + "\n" +
		path + `:7: job "c" runs on "windows-2025"` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.String() != hint {
		t.Errorf("stderr %q, want %q", stderr.String(), hint)
	}
}

func TestRunMissingFileStillChecksTheRest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join("testdata", "missing.yml")
	x64 := filepath.Join("testdata", "scalar.yml")

	code := run([]string{missing, x64}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), missing+": open ") || !strings.HasSuffix(stderr.String(), hint) {
		t.Errorf("stderr %q, want the open error, then the hint", stderr.String())
	}
	if want := x64 + `:3: job "a" runs on "ubuntu-24.04"` + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestRunMissingFileAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{filepath.Join("testdata", "missing.yml")}, &stdout, &stderr); code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		file string   // in testdata/
		want []string // "job label line"
	}{
		{"scalar.yml", []string{"a ubuntu-24.04 3"}},
		{"quoted-with-comment.yml", []string{"a windows-2025 3"}},
		{"flow-list.yml", []string{"a ubuntu-24.04 3"}},
		{"block-list.yml", []string{"a ubuntu-24.04 5"}},
		{"group-labels.yml", []string{"a ubuntu-24.04 5"}},
		{"several-jobs.yml", []string{"a ubuntu-24.04 3", "c windows-2025 7"}},
		{"x64-labels.yml", []string{
			"a ubuntu-latest 3",
			"b ubuntu-slim 5",
			"c windows-latest 7",
			"d macos-15-intel 9",
			"e macos-15-large 11",
			"f macos-13 13",
		}},
		{"arm.yml", nil},
		{"x64-outside-runs-on.yml", nil},
		{"expression.yml", nil},
		{"allow-marker.yml", nil},
		{"reusable-workflow-call.yml", nil},
		{"empty.yml", nil},
		// Self-hosted runners aren't billed per minute, so their arch is the
		// owner's choice. An exact label only: a lookalike doesn't count.
		{"self-hosted.yml", []string{"d ubuntu-24.04 11"}},
		// Aliases are followed and reported where they are used: the alias's
		// line, or the job's key for a job reused whole.
		{"alias-scalar.yml", []string{"a ubuntu-24.04 5"}},
		{"alias-reused-job.yml", []string{"test ubuntu-24.04 3", "alt-test ubuntu-24.04 4"}},
		{"alias-list-item.yml", []string{"a windows-2025 7", "b windows-2025 9"}},
		{"alias-labels.yml", []string{"a ubuntu-24.04 5", "b ubuntu-24.04 9", "c ubuntu-24.04 11"}},
		// A marker on the alias exempts that use; one on the anchor, every use.
		{"alias-allow.yml", []string{"c ubuntu-24.04 10", "g windows-2025 15"}},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			findings, err := checkFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			var got []string
			for _, f := range findings {
				got = append(got, fmt.Sprint(f.job, " ", f.label, " ", f.line))
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckInvalidYAML(t *testing.T) {
	if _, err := check(strings.NewReader("jobs: [unclosed")); err == nil {
		t.Fatal("want an error for invalid YAML")
	}
}
