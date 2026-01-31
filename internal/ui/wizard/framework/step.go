package framework

import tea "github.com/charmbracelet/bubbletea"

// StepResult indicates what action to take after a step update.
type StepResult int

const (
	// StepContinue means stay on the current step.
	StepContinue StepResult = iota
	// StepAdvance means move to the next step.
	StepAdvance
	// StepBack means move to the previous step.
	StepBack
)

// StepValue holds the result of a completed step.
type StepValue struct {
	Key   string      // Field name (e.g., "Branch")
	Label string      // Display value (e.g., "feature-branch")
	Raw   interface{} // Actual value (can be string, bool, []string, etc.)
}

// Step is the interface for wizard steps.
type Step interface {
	// ID returns a unique identifier for this step.
	ID() string

	// Title returns the display title for the step tab.
	Title() string

	// Init returns an initial command when entering this step.
	Init() tea.Cmd

	// Update handles key events and returns the updated step,
	// a command to run, and a result indicating navigation.
	Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult)

	// View renders the step content.
	View() string

	// Help returns the help text for this step.
	Help() string

	// Value returns the step's current value for summary display.
	Value() StepValue

	// IsComplete returns true if the step has a valid selection.
	IsComplete() bool

	// Reset clears the step's selection/input.
	Reset()
}

// Option represents a selectable item in list-based steps.
type Option struct {
	Label       string      // Display text
	Value       interface{} // Actual value
	Description string      // Optional description (for disabled reason)
	Disabled    bool        // Whether option is disabled/unselectable
}
