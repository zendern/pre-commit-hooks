package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const (
	hint      = "Set timeout-minutes so a hung run is cancelled before GitHub's 6-hour default.\n"
	stepsHint = "Keep each step's timeout-minutes below its job's, or the job is cancelled first.\n"
)

func TestRunClean(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{filepath.Join("testdata", "job-with-timeout.yml")}, &stdout, &stderr)

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

func TestRunJobMissingTimeout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join("testdata", "several-jobs.yml")

	code := run([]string{path}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	want := path + `:2: job "a" has no timeout-minutes` + "\n" +
		path + `:7: job "c" has no timeout-minutes` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.String() != hint {
		t.Errorf("stderr %q, want only the job hint %q", stderr.String(), hint)
	}
}

func TestRunStepFindings(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join("testdata", "step-position-label.yml")
	notBelow := filepath.Join("testdata", "step-timeout-not-below-job.yml")

	code := run([]string{"--steps", missing, notBelow}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	want := missing + `:5: step #1 in job "a" has no timeout-minutes` + "\n" +
		notBelow + `:8: step "Test" timeout-minutes 30 is not below job "a" timeout-minutes 30` + "\n" +
		notBelow + `:11: step "Soak" timeout-minutes 45 is not below job "a" timeout-minutes 30` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.String() != hint+stepsHint {
		t.Errorf("stderr %q, want both hints", stderr.String())
	}
}

func TestRunMissingFileStillChecksTheRest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join("testdata", "missing.yml")
	jobMissing := filepath.Join("testdata", "job-missing-timeout.yml")

	code := run([]string{missing, jobMissing}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), missing+": open ") || !strings.HasSuffix(stderr.String(), hint) {
		t.Errorf("stderr %q, want the open error, then the hint", stderr.String())
	}
	if want := jobMissing + `:2: job "a" has no timeout-minutes` + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestRunMissingFileAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{filepath.Join("testdata", "missing.yml")}, &stdout, &stderr); code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--bogus"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("code %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined: -bogus") {
		t.Errorf("stderr %q, want the flag error", stderr.String())
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		file  string // in testdata/
		steps bool
		// "job step line" for a missing timeout, where step is "-" for a job;
		// "job step line step>=job" for a step timeout the job's always beats.
		want []string
	}{
		{"job-missing-timeout.yml", false, []string{"a - 2"}},
		{"job-with-timeout.yml", false, nil},
		{"expression-timeout.yml", false, nil},
		{"empty-timeout.yml", false, []string{"a - 2"}},
		{"several-jobs.yml", false, []string{"a - 2", "c - 7"}},
		{"reusable-workflow-call.yml", false, nil},
		{"steps-without-flag.yml", false, nil},
		{"steps-checked.yml", true, []string{`a "actions/checkout@v5" 5`, `a "Build" 6`, `a "lint" 10`}},
		{"step-position-label.yml", true, []string{"a #1 5"}},
		{"job-and-steps-both-missing.yml", true, []string{"a - 2", "a #1 4"}},
		{"allow-marker.yml", true, nil},
		{"step-timeout-not-below-job.yml", true, []string{`a "Test" 8 30>=30`, `a "Soak" 11 45>=30`}},
		{"step-timeout-not-below-job.yml", false, nil},
		{"job-timeout-expression.yml", true, nil},
		{"no-jobs.yml", false, nil},
		{"empty.yml", false, nil},
		{"comment-only.yml", false, nil},
		{"steps-missing.yml", true, nil},
		{"step-flow-allow.yml", true, []string{"a #2 7"}},
		// Aliases are followed and reported where they are used: the alias's
		// line, or the job's key for a job reused whole.
		{"alias-reused-job.yml", false, []string{"test - 2", "alt-test - 6"}},
		{"alias-reused-job.yml", true, []string{"test - 2", "test #1 5", "alt-test - 6", "alt-test #1 6"}},
		{"alias-timeout.yml", true, []string{"a - 5", `b #1 12 30>=30`}},
		{"alias-steps.yml", true, []string{
			`a "actions/checkout@v5" 6`, `b "actions/checkout@v5" 10`, `c "actions/checkout@v5" 15`,
		}},
		{"alias-allow.yml", false, []string{"test - 2"}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s steps=%t", tt.file, tt.steps), func(t *testing.T) {
			findings, err := checkFile(filepath.Join("testdata", tt.file), tt.steps)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			var got []string
			for _, f := range findings {
				step := "-"
				if f.step != "" {
					step = f.step
				}
				g := fmt.Sprint(f.job, " ", step, " ", f.line)
				if f.stepTimeout != 0 {
					g += fmt.Sprintf(" %g>=%g", f.stepTimeout, f.jobTimeout)
				}
				got = append(got, g)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckInvalidYAML(t *testing.T) {
	if _, err := check(strings.NewReader("jobs: [unclosed"), false); err == nil {
		t.Fatal("want an error for invalid YAML")
	}
}
