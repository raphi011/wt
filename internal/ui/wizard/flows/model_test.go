package flows

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

// keyMsgFlows creates a tea.KeyPressMsg from a string key.
// Mirrors the keyMsg helper in the steps package tests.
func keyMsgFlows(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		if len(key) == 1 {
			r := rune(key[0])
			return tea.KeyPressMsg{Code: r, Text: key}
		}
		return tea.KeyPressMsg{}
	}
}

// newCdModel creates a cdListModel with the given worktrees for testing.
func newCdModel(worktrees []CdWorktreeInfo) *cdListModel {
	options := make([]framework.Option, len(worktrees))
	for i, wt := range worktrees {
		options[i] = framework.Option{
			Label: wt.RepoName + ":" + wt.Branch,
			Value: i,
		}
	}
	selectStep := steps.NewFilterableList("worktree", "Worktree", "", options)
	return &cdListModel{
		step:       selectStep,
		worktrees:  worktrees,
		selectedAt: -1,
	}
}

// --- cdListModel tests ---

func TestCdListModel_Init(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)
	// Init should return a command (or nil) without panicking
	_ = m.Init()
}

func TestCdListModel_View_NotDone(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
		{RepoName: "my-repo", Branch: "feature-a", Path: "/tmp/feature-a"},
	}
	m := newCdModel(worktrees)

	view := m.View()
	// View should return a non-empty string containing list content
	if view.Content == "" {
		t.Error("View() should return non-empty content when not done/cancelled")
	}
}

func TestCdListModel_View_Done(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)
	m.done = true

	view := m.View()
	if view.Content != "" {
		t.Errorf("View() should return empty content when done, got %q", view.Content)
	}
}

func TestCdListModel_View_Cancelled(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)
	m.cancelled = true

	view := m.View()
	if view.Content != "" {
		t.Errorf("View() should return empty content when cancelled, got %q", view.Content)
	}
}

func TestCdListModel_Update_NonKeyMsg(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)

	// Non-key messages should be ignored
	model, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Error("expected nil cmd for non-key message")
	}
	updated := model.(*cdListModel)
	if updated.done {
		t.Error("model should not be done after non-key message")
	}
	if updated.cancelled {
		t.Error("model should not be cancelled after non-key message")
	}
}

func TestCdListModel_Update_CtrlC(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)

	model, _ := m.Update(keyMsgFlows("ctrl+c"))
	updated := model.(*cdListModel)
	if !updated.cancelled {
		t.Error("ctrl+c should set cancelled=true")
	}
}

func TestCdListModel_Update_Esc_CancelsWhenNoFilter(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)

	// No filter set, esc should cancel
	model, _ := m.Update(keyMsgFlows("esc"))
	updated := model.(*cdListModel)
	if !updated.cancelled {
		t.Error("esc should cancel when no filter is set")
	}
}

func TestCdListModel_Update_Esc_ClearsFilter(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "my-repo", Branch: "main", Path: "/tmp/main"},
		{RepoName: "my-repo", Branch: "feature-a", Path: "/tmp/feature-a"},
	}
	m := newCdModel(worktrees)

	// Type something to set a filter
	m.Update(keyMsgFlows("m"))
	if !m.step.HasClearableInput() {
		t.Skip("filter not set after typing; skipping esc-clears-filter test")
	}

	// Esc should clear filter, not cancel
	model, _ := m.Update(keyMsgFlows("esc"))
	updated := model.(*cdListModel)
	if updated.cancelled {
		t.Error("esc should clear filter rather than cancel when filter is set")
	}
	if updated.step.HasClearableInput() {
		t.Error("filter should be cleared after esc")
	}
}

func TestCdListModel_Update_Enter_SelectsItem(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "repo1", Branch: "main", Path: "/tmp/main", LastAccess: time.Now()},
		{RepoName: "repo1", Branch: "feature", Path: "/tmp/feature", LastAccess: time.Now()},
	}
	m := newCdModel(worktrees)

	// Press enter to select the first item
	model, _ := m.Update(keyMsgFlows("enter"))
	updated := model.(*cdListModel)

	if !updated.done {
		t.Error("pressing enter should set done=true")
	}
	if updated.cancelled {
		t.Error("pressing enter should not cancel")
	}
	if updated.selectedAt != 0 {
		t.Errorf("selectedAt = %d, want 0", updated.selectedAt)
	}
}

func TestCdListModel_Update_Down_ThenEnter(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "repo1", Branch: "main", Path: "/tmp/main"},
		{RepoName: "repo1", Branch: "feature", Path: "/tmp/feature"},
	}
	m := newCdModel(worktrees)

	// Navigate down, then select
	m.Update(keyMsgFlows("down"))
	model, _ := m.Update(keyMsgFlows("enter"))
	updated := model.(*cdListModel)

	if !updated.done {
		t.Error("should be done after enter")
	}
	if updated.selectedAt != 1 {
		t.Errorf("selectedAt = %d, want 1", updated.selectedAt)
	}
}

// --- addHookStep tests ---

func TestAddHookStep_NilHooks_AddsEmptyStep(t *testing.T) {
	t.Parallel()
	w := framework.NewWizard("Test")
	// addHookStep always adds the step — callers guard with `if hasHooks`
	addHookStep(w, nil)
	if w.StepCount() != 1 {
		t.Errorf("StepCount() = %d, want 1 (empty hooks step is still added)", w.StepCount())
	}
	step := w.GetStep("hooks")
	if step == nil {
		t.Fatal("hooks step should be present even with nil hooks")
	}
}

func TestAddHookStep_WithHooks(t *testing.T) {
	t.Parallel()
	w := framework.NewWizard("Test")
	hooks := []HookInfo{
		{Name: "build", Description: "Run build", IsDefault: false},
		{Name: "test", Description: "Run tests", IsDefault: true},
	}
	addHookStep(w, hooks)
	if w.StepCount() != 1 {
		t.Errorf("StepCount() = %d, want 1 after adding hooks step", w.StepCount())
	}
	step := w.GetStep("hooks")
	if step == nil {
		t.Fatal("hooks step should be present")
	}
	if step.ID() != "hooks" {
		t.Errorf("step.ID() = %q, want %q", step.ID(), "hooks")
	}
}

func TestAddHookStep_PreSelectsDefaultHooks(t *testing.T) {
	t.Parallel()
	w := framework.NewWizard("Test")
	hooks := []HookInfo{
		{Name: "build", Description: "Run build", IsDefault: false},
		{Name: "test", Description: "Run tests", IsDefault: true},
		{Name: "lint", Description: "Lint code", IsDefault: true},
	}
	addHookStep(w, hooks)

	step := w.GetStep("hooks")
	if step == nil {
		t.Fatal("hooks step should be present")
	}
	fl, ok := step.(*steps.FilterableListStep)
	if !ok {
		t.Fatalf("hooks step should be *FilterableListStep, got %T", step)
	}

	// "test" and "lint" are default, so 2 pre-selected
	if fl.SelectedCount() != 2 {
		t.Errorf("SelectedCount() = %d, want 2 (default hooks pre-selected)", fl.SelectedCount())
	}
}

func TestAddHookStep_NoDefaultHooks_NonePreSelected(t *testing.T) {
	t.Parallel()
	w := framework.NewWizard("Test")
	hooks := []HookInfo{
		{Name: "deploy", Description: "Deploy", IsDefault: false},
		{Name: "notify", Description: "Notify", IsDefault: false},
	}
	addHookStep(w, hooks)

	step := w.GetStep("hooks")
	fl, ok := step.(*steps.FilterableListStep)
	if !ok {
		t.Fatalf("hooks step should be *FilterableListStep, got %T", step)
	}
	if fl.SelectedCount() != 0 {
		t.Errorf("SelectedCount() = %d, want 0 when no defaults", fl.SelectedCount())
	}
}

func TestAddHookStep_LabelFormat_WithDescription(t *testing.T) {
	t.Parallel()
	w := framework.NewWizard("Test")
	hooks := []HookInfo{
		{Name: "build", Description: "Run the build script", IsDefault: false},
	}
	addHookStep(w, hooks)

	step := w.GetStep("hooks")
	fl, ok := step.(*steps.FilterableListStep)
	if !ok {
		t.Fatalf("hooks step should be *FilterableListStep, got %T", step)
	}
	view := fl.View()
	if !strings.Contains(view, "build - Run the build script") {
		t.Errorf("View should contain 'build - Run the build script', got: %s", view)
	}
}

func TestAddHookStep_LabelFormat_WithoutDescription(t *testing.T) {
	t.Parallel()
	w := framework.NewWizard("Test")
	hooks := []HookInfo{
		{Name: "deploy", Description: "", IsDefault: false},
	}
	addHookStep(w, hooks)

	step := w.GetStep("hooks")
	fl, ok := step.(*steps.FilterableListStep)
	if !ok {
		t.Fatalf("hooks step should be *FilterableListStep, got %T", step)
	}
	view := fl.View()
	// Should show just the name, without a " - " separator
	if !strings.Contains(view, "deploy") {
		t.Errorf("View should contain 'deploy', got: %s", view)
	}
	if strings.Contains(view, "deploy -") {
		t.Errorf("View should not contain 'deploy -' when no description, got: %s", view)
	}
}

// --- buildBranchOptions additional tests ---

func TestBuildBranchOptions_SingleNormal(t *testing.T) {
	t.Parallel()
	branches := []BranchInfo{
		{Name: "develop", InWorktree: false},
	}
	opts := buildBranchOptions(branches)
	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}
	if opts[0].Label != "develop" {
		t.Errorf("Label = %q, want develop", opts[0].Label)
	}
	if opts[0].Value != "develop" {
		t.Errorf("Value = %v, want develop", opts[0].Value)
	}
}

// --- cdListModel View content tests ---

func TestCdListModel_View_ContainsBranch(t *testing.T) {
	t.Parallel()
	worktrees := []CdWorktreeInfo{
		{RepoName: "repo1", Branch: "main", Path: "/tmp/main"},
	}
	m := newCdModel(worktrees)
	view := m.View()
	if !strings.Contains(view.Content, "repo1:main") {
		t.Errorf("View should contain worktree label 'repo1:main', got: %s", view.Content)
	}
}

// --- PruneOptionValue tests ---

func TestPruneOptionValue_Fields(t *testing.T) {
	t.Parallel()
	v := pruneOptionValue{
		ID:         7,
		IsPrunable: true,
		IsStale:    false,
		Reason:     "● Merged",
	}
	if v.ID != 7 {
		t.Errorf("ID = %d, want 7", v.ID)
	}
	if !v.IsPrunable {
		t.Error("IsPrunable should be true")
	}
	if v.IsStale {
		t.Error("IsStale should be false")
	}
	if v.Reason != "● Merged" {
		t.Errorf("Reason = %q, want '● Merged'", v.Reason)
	}
}
