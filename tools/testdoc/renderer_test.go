package main

import (
	"errors"
	"strings"
	"testing"
)

// failingWriter fails once more than limit writes were made.
// A negative limit never fails.
type failingWriter struct {
	limit  int
	writes int
	sb     strings.Builder
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.limit >= 0 && w.writes >= w.limit {
		return 0, errors.New("write failed")
	}
	w.writes++
	return w.sb.Write(p)
}

func renderTestPackages() []TestPackage {
	return []TestPackage{{
		Name: "cmd/wt",
		Files: []TestFile{{
			Name: "checkout_integration_test.go",
			Tests: []TestFunc{
				{Name: "TestCheckout_NewBranch", Doc: "TestCheckout_NewBranch creates a | branch."},
				{Name: "TestCd_BasicNavigation"},
			},
		}},
	}}
}

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	w := &failingWriter{limit: -1}
	if err := RenderMarkdown(w, renderTestPackages()); err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}

	got := w.sb.String()
	for _, want := range []string{
		"# Test Documentation",
		"| **Total** | **2** |",
		"## wt checkout",
		"## wt cd",
		"| `TestCheckout_NewBranch` |",
		`\|`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderMarkdown_WriteError(t *testing.T) {
	t.Parallel()

	ok := &failingWriter{limit: -1}
	if err := RenderMarkdown(ok, renderTestPackages()); err != nil {
		t.Fatalf("RenderMarkdown failed: %v", err)
	}

	// Fail at every write position in turn
	for limit := range ok.writes {
		w := &failingWriter{limit: limit}
		if err := RenderMarkdown(w, renderTestPackages()); err == nil {
			t.Errorf("RenderMarkdown with write %d failing returned no error", limit+1)
		}
	}
}
