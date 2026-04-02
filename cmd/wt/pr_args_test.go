package main

import (
	"testing"
)

func TestParsePrCheckoutArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantNumber int
		wantRepo   string
		wantErr    bool
	}{
		{"just number", []string{"123"}, 123, "", false},
		{"repo and number", []string{"myrepo", "456"}, 456, "myrepo", false},
		{"org/repo and number", []string{"org/repo", "789"}, 789, "org/repo", false},
		{"invalid number", []string{"abc"}, 0, "", true},
		{"repo with invalid number", []string{"myrepo", "abc"}, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prNumber, repoArg, err := parsePrCheckoutArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if prNumber != tt.wantNumber {
				t.Errorf("prNumber = %d, want %d", prNumber, tt.wantNumber)
			}
			if repoArg != tt.wantRepo {
				t.Errorf("repoArg = %q, want %q", repoArg, tt.wantRepo)
			}
		})
	}
}
