package tui

import (
	"rakun/internal/taskrun"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestProgressView(t *testing.T) {
	testCases := []struct {
		name     string
		model    model
		expected [][]string
	}{
		{
			name: "renders recent outcome active task and footer",
			model: model{
				completed: 1,
				frame:     1,
				height:    6,
				jobs:      2,
				order:     []string{"done", "active"},
				taskStates: map[string]*taskState{
					"done": {
						completed: true,
						status:    "Done",
						title:     "/tmp/repo-one",
					},
					"active": {
						progress: 0.4,
						status:   "Downloading refs",
						title:    "/tmp/repo-two",
					},
				},
				total: 3,
			},
			expected: [][]string{
				{"✔ repo-one  Done"},
				{"repo-two", "40%", "Downloading refs"},
				{""},
				{"⠙ Processing... (1/3)"},
			},
		},
		{
			name: "omits recent outcomes when height is exhausted",
			model: model{
				completed: 1,
				frame:     0,
				height:    2,
				jobs:      1,
				order:     []string{"done", "active"},
				taskStates: map[string]*taskState{
					"done": {
						completed: true,
						status:    "Done",
						title:     "/tmp/repo-one",
					},
					"active": {
						progress: 0.25,
						status:   "Running",
						title:    "/tmp/repo-two",
					},
				},
				total: 3,
			},
			expected: [][]string{
				{"repo-two", "25%", "Running"},
				{""},
				{"⠋ Processing... (1/3)"},
			},
		},
		{
			name: "renders canceling footer without task rows",
			model: model{
				cancelRequested: true,
				frame:           2,
				height:          4,
				jobs:            2,
				order:           []string{},
				taskStates:      map[string]*taskState{},
				total:           5,
			},
			expected: [][]string{
				{"⠹ Processing... (0/5)  canceling..."},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := plainLines(testCase.model.progressView())
			if len(got) != len(testCase.expected) {
				t.Fatalf("unexpected line count: got %d want %d (%#v)", len(got), len(testCase.expected), got)
			}
			for index, fragments := range testCase.expected {
				for _, fragment := range fragments {
					if !strings.Contains(got[index], fragment) {
						t.Fatalf("expected fragment %q in line %d: %q", fragment, index, got[index])
					}
				}
			}
		})
	}
}

func TestSummaryView(t *testing.T) {
	testCases := []struct {
		name     string
		summary  taskrun.Summary
		expected []string
	}{
		{
			name: "renders duration without zero-value stats",
			summary: taskrun.Summary{
				Duration: 1200 * time.Millisecond,
			},
			expected: []string{
				"completed in 1.2s",
			},
		},
		{
			name: "renders non-zero stats in order",
			summary: taskrun.Summary{
				Succeeded: 2,
				Changed:   1,
				Unchanged: 3,
				Canceled:  4,
				Failed:    5,
				Duration:  2500 * time.Millisecond,
			},
			expected: []string{
				"completed in 2.5s",
				"",
				"● 2 succeeded",
				"● 1 changed",
				"● 3 unchanged",
				"● 4 canceled",
				"● 5 failed",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newModel(0, 0, nil)
			m.summary = &testCase.summary

			got := plainLines(m.summaryView())
			if len(got) != len(testCase.expected) {
				t.Fatalf("unexpected line count: got %d want %d (%#v)", len(got), len(testCase.expected), got)
			}
			for index, want := range testCase.expected {
				if got[index] != want {
					t.Fatalf("unexpected line %d: got %q want %q", index, got[index], want)
				}
			}
		})
	}
}

func plainLines(value string) []string {
	plain := ansi.Strip(strings.TrimSuffix(value, "\n"))
	if plain == "" {
		return nil
	}
	return strings.Split(plain, "\n")
}
