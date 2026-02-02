// Package framework provides the core wizard orchestration system.
//
// A wizard is a multi-step interactive flow that guides users through
// complex operations. It manages step navigation, skip conditions,
// callbacks, and summary display.
package framework

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// Wizard orchestrates a multi-step interactive flow.
type Wizard struct {
	title          string
	steps          []Step
	stepIndex      map[string]int // id -> index
	currentStep    int
	skipConditions map[string]func(*Wizard) bool // step id -> skip condition
	onComplete     map[string]func(*Wizard)      // step id -> callback
	infoLine       func(*Wizard) string          // dynamic info line
	summaryTitle   string
	skipSummary    bool // if true, skip summary and finish after last step
	done           bool
	cancelled      bool
	width          int
	height         int
	confirmedSteps map[string]bool // tracks steps user has confirmed (advanced past)
}

// NewWizard creates a new wizard with the given title.
func NewWizard(title string) *Wizard {
	return &Wizard{
		title:          title,
		steps:          nil,
		stepIndex:      make(map[string]int),
		skipConditions: make(map[string]func(*Wizard) bool),
		onComplete:     make(map[string]func(*Wizard)),
		summaryTitle:   "Review and confirm",
		width:          60,
		height:         20,
		confirmedSteps: make(map[string]bool),
	}
}

// AddStep adds a step to the wizard.
func (w *Wizard) AddStep(step Step) *Wizard {
	w.stepIndex[step.ID()] = len(w.steps)
	w.steps = append(w.steps, step)
	return w
}

// SkipWhen sets a condition for skipping a step.
func (w *Wizard) SkipWhen(stepID string, condition func(*Wizard) bool) *Wizard {
	w.skipConditions[stepID] = condition
	return w
}

// OnComplete sets a callback to run when a step completes.
func (w *Wizard) OnComplete(stepID string, callback func(*Wizard)) *Wizard {
	w.onComplete[stepID] = callback
	return w
}

// WithSummary sets the summary step title.
func (w *Wizard) WithSummary(title string) *Wizard {
	w.summaryTitle = title
	return w
}

// WithInfoLine sets a dynamic info line function.
func (w *Wizard) WithInfoLine(fn func(*Wizard) string) *Wizard {
	w.infoLine = fn
	return w
}

// WithSkipSummary sets whether to skip the summary step and finish after the last step.
// Useful for single-step wizards where a confirmation summary is unnecessary.
func (w *Wizard) WithSkipSummary(skip bool) *Wizard {
	w.skipSummary = skip
	return w
}

// GetStep returns a step by ID.
func (w *Wizard) GetStep(id string) Step {
	if idx, ok := w.stepIndex[id]; ok {
		return w.steps[idx]
	}
	return nil
}

// GetValue returns a step's value by ID.
func (w *Wizard) GetValue(id string) StepValue {
	if step := w.GetStep(id); step != nil {
		return step.Value()
	}
	return StepValue{}
}

// GetString returns a step's value as a string.
func (w *Wizard) GetString(id string) string {
	v := w.GetValue(id)
	if s, ok := v.Raw.(string); ok {
		return s
	}
	return v.Label
}

// GetBool returns a step's value as a bool.
func (w *Wizard) GetBool(id string) bool {
	v := w.GetValue(id)
	if b, ok := v.Raw.(bool); ok {
		return b
	}
	return false
}

// GetStrings returns a step's value as a string slice.
func (w *Wizard) GetStrings(id string) []string {
	v := w.GetValue(id)
	if arr, ok := v.Raw.([]interface{}); ok {
		strs := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				strs = append(strs, s)
			}
		}
		return strs
	}
	if arr, ok := v.Raw.([]string); ok {
		return arr
	}
	return nil
}

// IsCancelled returns true if the wizard was cancelled.
func (w *Wizard) IsCancelled() bool {
	return w.cancelled
}

// Run executes the wizard and returns when complete or cancelled.
// The TUI renders to stderr so stdout remains available for piping
// (e.g., cd $(wt cd -i) works correctly).
func (w *Wizard) Run() (*Wizard, error) {
	if len(w.steps) == 0 {
		return w, fmt.Errorf("wizard has no steps")
	}

	// Detect color profile for stderr (handles piped output, NO_COLOR, etc.)
	profile := colorprofile.Detect(os.Stderr, os.Environ())

	// Run with output to stderr so stdout can be piped
	// (e.g., cd $(wt cd -i) redirects stdout but stderr remains a TTY)
	p := tea.NewProgram(w,
		tea.WithOutput(os.Stderr),
		tea.WithColorProfile(profile),
	)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := finalModel.(*Wizard)
	return result, nil
}

// BubbleTea Model interface

func (w *Wizard) Init() tea.Cmd {
	if len(w.steps) > 0 {
		// Skip to first non-skipped, incomplete step
		w.currentStep = w.findNextIncompleteStep(-1)
		if w.currentStep < 0 {
			// All steps complete, go to summary
			w.currentStep = len(w.steps)
		}
		// Mark all complete steps before currentStep as confirmed
		// (they were pre-filled and we're skipping past them)
		for i := 0; i < w.currentStep && i < len(w.steps); i++ {
			step := w.steps[i]
			// Skip steps that are conditionally skipped
			if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
				continue
			}
			if step.IsComplete() {
				w.confirmedSteps[step.ID()] = true
			}
		}
		if w.currentStep < len(w.steps) {
			return w.steps[w.currentStep].Init()
		}
	}
	return nil
}

func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		return w, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			// If step has clearable input, clear it first
			if w.currentStep < len(w.steps) {
				step := w.steps[w.currentStep]
				if step.HasClearableInput() {
					cmd := step.ClearInput()
					return w, cmd
				}
			}
			// No input to clear, cancel wizard
			w.cancelled = true
			w.done = true
			return w, tea.Quit
		}

		// Check if we're on summary step (virtual step at the end)
		if w.currentStep >= len(w.steps) {
			return w.handleSummaryInput(msg)
		}

		// Delegate to current step
		step := w.steps[w.currentStep]
		newStep, cmd, result := step.Update(msg)
		w.steps[w.currentStep] = newStep

		switch result {
		case StepSubmitIfReady:
			// Mark step as confirmed
			w.confirmedSteps[step.ID()] = true
			// Run completion callback
			if cb, ok := w.onComplete[step.ID()]; ok {
				cb(w)
			}
			// Find next incomplete step or go to summary
			next := w.findNextStep(w.currentStep)
			if next < 0 {
				// No more steps - go to summary or submit if skipSummary
				if w.skipSummary {
					w.done = true
					return w, tea.Quit
				}
				w.currentStep = len(w.steps) // Go to summary
			} else {
				w.currentStep = next
				return w, w.steps[w.currentStep].Init()
			}
		case StepAdvance:
			// Mark step as confirmed (user advanced past it)
			w.confirmedSteps[step.ID()] = true
			// Run completion callback
			if cb, ok := w.onComplete[step.ID()]; ok {
				cb(w)
			}
			// Move to next step
			next := w.findNextStep(w.currentStep)
			if next < 0 {
				// No more steps
				if w.skipSummary {
					// Skip summary, finish immediately
					w.done = true
					return w, tea.Quit
				}
				// Go to summary
				w.currentStep = len(w.steps)
			} else {
				w.currentStep = next
				return w, w.steps[w.currentStep].Init()
			}
		case StepBack:
			prev := w.findPrevStep(w.currentStep)
			if prev >= 0 {
				w.currentStep = prev
			}
		}

		return w, cmd
	}

	return w, nil
}

func (w *Wizard) View() tea.View {
	if w.done {
		return tea.NewView("")
	}

	var b strings.Builder

	// Title
	b.WriteString(TitleStyle().Render(w.title))
	b.WriteString("\n\n")

	// Info line
	if w.infoLine != nil {
		if info := w.infoLine(w); info != "" {
			b.WriteString(InfoStyle().Render(info))
			b.WriteString("\n\n")
		}
	}

	// Step tabs (skip if single step with no summary)
	if !(len(w.steps) == 1 && w.skipSummary) {
		b.WriteString(w.renderStepTabs())
		b.WriteString("\n\n")
	}

	// Current step content or summary
	if w.currentStep >= len(w.steps) {
		b.WriteString(w.renderSummary())
	} else {
		b.WriteString(w.steps[w.currentStep].View())
	}
	b.WriteString("\n")

	// Help text
	if w.currentStep >= len(w.steps) {
		b.WriteString(HelpStyle().Render("← back • enter confirm • esc cancel"))
	} else {
		b.WriteString(HelpStyle().Render(w.steps[w.currentStep].Help()))
	}

	return tea.NewView(BorderStyle().Render(b.String()))
}

func (w *Wizard) handleSummaryInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		w.done = true
		return w, tea.Quit
	case "left":
		// Go back to last step
		prev := w.findPrevStep(len(w.steps))
		if prev >= 0 {
			w.currentStep = prev
		}
	}
	return w, nil
}

func (w *Wizard) renderStepTabs() string {
	var tabs []string
	displayNum := 1

	for i, step := range w.steps {
		// Check if step should be skipped
		if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
			continue
		}

		isActive := i == w.currentStep
		isConfirmed := w.confirmedSteps[step.ID()]
		label := fmt.Sprintf("%d. %s", displayNum, step.Title())
		displayNum++

		var tabText string
		if isActive && isConfirmed {
			// Current step that's also confirmed (went back to edit)
			checkmark := StepCheckStyle().Render("✓ ")
			tabText = checkmark + StepActiveStyle().Render(label)
		} else if isActive {
			tabText = "  " + StepActiveStyle().Render(label)
		} else if isConfirmed {
			checkmark := StepCheckStyle().Render("✓ ")
			tabText = checkmark + StepCompletedStyle().Render(label)
		} else {
			tabText = "  " + StepInactiveStyle().Render(label)
		}

		tabs = append(tabs, tabText)
	}

	// Add summary tab (unless skipSummary is set)
	if !w.skipSummary {
		summaryLabel := fmt.Sprintf("%d. Summary", displayNum)
		isSummaryActive := w.currentStep >= len(w.steps)
		if isSummaryActive {
			tabs = append(tabs, "  "+StepActiveStyle().Render(summaryLabel))
		} else {
			tabs = append(tabs, "  "+StepInactiveStyle().Render(summaryLabel))
		}
	}

	return strings.Join(tabs, StepArrowStyle().Render(" → "))
}

func (w *Wizard) renderSummary() string {
	var b strings.Builder
	b.WriteString(w.summaryTitle + ":\n\n")

	for _, step := range w.steps {
		// Skip steps that are skipped
		if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
			continue
		}

		v := step.Value()
		if v.Label == "" {
			continue
		}

		b.WriteString(SummaryLabelStyle().Render(step.Title()+": ") +
			SummaryValueStyle().Render(v.Label) + "\n")
	}

	b.WriteString("\n" + OptionNormalStyle().Render("Press enter to confirm, ← to go back"))
	return b.String()
}

func (w *Wizard) findNextStep(from int) int {
	for i := from + 1; i < len(w.steps); i++ {
		step := w.steps[i]
		if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
			continue
		}
		return i
	}
	return -1
}

func (w *Wizard) findPrevStep(from int) int {
	for i := from - 1; i >= 0; i-- {
		step := w.steps[i]
		if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
			continue
		}
		return i
	}
	return -1
}

func (w *Wizard) findNextIncompleteStep(from int) int {
	for i := from + 1; i < len(w.steps); i++ {
		step := w.steps[i]
		if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
			continue
		}
		if !step.IsComplete() {
			return i
		}
	}
	return -1
}

// CurrentStepID returns the current step's ID, or "summary" if on summary.
func (w *Wizard) CurrentStepID() string {
	if w.currentStep >= len(w.steps) {
		return "summary"
	}
	return w.steps[w.currentStep].ID()
}

// SetCurrentStep sets the current step by ID.
func (w *Wizard) SetCurrentStep(id string) {
	if idx, ok := w.stepIndex[id]; ok {
		w.currentStep = idx
	}
}

// StepCount returns the number of steps (excluding summary).
func (w *Wizard) StepCount() int {
	return len(w.steps)
}

// AllStepsComplete returns true if all steps have values.
func (w *Wizard) AllStepsComplete() bool {
	for _, step := range w.steps {
		if cond, ok := w.skipConditions[step.ID()]; ok && cond(w) {
			continue
		}
		if !step.IsComplete() {
			return false
		}
	}
	return true
}
