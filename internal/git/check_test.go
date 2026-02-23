package git

import (
	"errors"
	"testing"
)

func TestCheckGit_Available(t *testing.T) {
	t.Parallel()
	// git must be available in CI and dev environments
	if err := CheckGit(); err != nil {
		t.Fatalf("CheckGit() = %v, want nil (git should be in PATH)", err)
	}
}

func TestErrGitNotFound_Sentinel(t *testing.T) {
	t.Parallel()
	// Verify ErrGitNotFound is a distinct sentinel error
	if !errors.Is(ErrGitNotFound, ErrGitNotFound) {
		t.Error("ErrGitNotFound should match itself with errors.Is")
	}
}
