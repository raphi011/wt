package steps

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// LoadingConfig separates background I/O from applying its result in Update.
type LoadingConfig[T any] struct {
	Context context.Context
	Key     func() (string, error)
	Fetch   func(context.Context, string) (T, error)
	// Apply updates the underlying step and returns whether the result is empty.
	Apply       func(T) bool
	LoadingText string
	EmptyText   string
}

type loadResult[T any] struct {
	stepID    string
	requestID uint64
	value     T
	err       error
}

// LoadingStep wraps a selector with cancellable loading, caching and retry.
// Only the returned command performs I/O; only Update/Init apply UI state.
type LoadingStep[T any] struct {
	framework.Step
	width, height int
	config        LoadingConfig[T]
	cache         map[string]T
	loadedKey     string
	ready         bool
	empty         bool
	err           error
	requestID     uint64
	cancel        context.CancelFunc
}

func NewLoadingStep[T any](step framework.Step, config LoadingConfig[T]) *LoadingStep[T] {
	if config.Context == nil {
		config.Context = context.Background()
	}
	return &LoadingStep[T]{Step: step, config: config, cache: make(map[string]T)}
}

func (s *LoadingStep[T]) Init() tea.Cmd { return s.load(false) }

func (s *LoadingStep[T]) load(refresh bool) tea.Cmd {
	repoKey, err := s.config.Key()
	if err != nil {
		s.Deactivate()
		s.ready, s.err = false, err
		return nil
	}
	if !refresh && repoKey == s.loadedKey && s.ready {
		return s.Step.Init()
	}
	s.Deactivate()
	s.loadedKey = repoKey
	s.ready, s.empty, s.err = false, false, nil
	s.Step.Reset()
	if refresh {
		delete(s.cache, repoKey)
	}
	if !refresh {
		if result, ok := s.cache[repoKey]; ok {
			s.empty = s.config.Apply(result)
			s.ready = true
			return s.Step.Init()
		}
	}
	ctx, cancel := context.WithCancel(s.config.Context)
	s.cancel = cancel
	s.requestID++
	requestID, stepID, fetch := s.requestID, s.ID(), s.config.Fetch
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return loadResult[T]{stepID: stepID, requestID: requestID, err: err}
		}
		value, err := fetch(ctx, repoKey)
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return loadResult[T]{stepID: stepID, requestID: requestID, value: value, err: err}
	}
}

// Deactivate stops obsolete requests when navigating away or closing the wizard.
func (s *LoadingStep[T]) Deactivate() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.requestID++
	}
}

func (s *LoadingStep[T]) Update(msg tea.Msg) (framework.Step, tea.Cmd, framework.StepResult) {
	defer s.resizeChild()
	if result, ok := msg.(loadResult[T]); ok {
		if result.stepID != s.ID() || result.requestID != s.requestID {
			return s, nil, framework.StepContinue
		}
		if s.cancel != nil {
			s.cancel()
			s.cancel = nil
		}
		s.err = result.err
		if result.err != nil {
			return s, nil, framework.StepContinue
		}
		s.cache[s.loadedKey] = result.value
		s.empty = s.config.Apply(result.value)
		s.ready = true
		return s, s.Step.Init(), framework.StepContinue
	}
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(msg, framework.Keys.Refresh) {
			return s, s.load(true), framework.StepContinue
		}
		if !s.ready {
			if key.Matches(msg, framework.Keys.ListBack) {
				return s, nil, framework.StepBack
			}
			return s, nil, framework.StepContinue
		}
	}
	if !s.ready {
		return s, nil, framework.StepContinue
	}
	step, cmd, result := s.Step.Update(msg)
	s.Step = step
	return s, cmd, result
}

func (s *LoadingStep[T]) View() string {
	if s.err != nil {
		return framework.ErrorStyle().Render(fmt.Sprintf("Could not load: %v", s.err)) + "\nRetry with Ctrl+R or go back to choose a repository.\n"
	}
	if !s.ready {
		return s.config.LoadingText + "\n"
	}
	if s.empty {
		return s.config.EmptyText + "\n\n" + s.Step.View()
	}
	return s.Step.View()
}

func (s *LoadingStep[T]) Help() string {
	if !s.ready {
		return framework.BindingHelp(framework.Keys.ListBack, framework.Keys.Refresh) + " • " + framework.CancellationHelp(false, "")
	}
	return s.Step.Help() + " • " + framework.BindingHelp(framework.Keys.Refresh)
}
func (s *LoadingStep[T]) IsComplete() bool        { return s.ready && s.Step.IsComplete() }
func (s *LoadingStep[T]) HasClearableInput() bool { return s.ready && s.Step.HasClearableInput() }
func (s *LoadingStep[T]) Value() framework.StepValue {
	if !s.ready {
		return framework.StepValue{Key: s.ID()}
	}
	return s.Step.Value()
}
func (s *LoadingStep[T]) Reset() {
	s.Deactivate()
	s.ready, s.empty, s.err = false, false, nil
	s.Step.Reset()
}

func (s *LoadingStep[T]) SetSize(width, height int) {
	s.width, s.height = width, height
	s.resizeChild()
}

func (s *LoadingStep[T]) resizeChild() {
	if step, ok := s.Step.(framework.SizedStep); ok && s.width > 0 {
		height := s.height
		if s.ready && s.empty {
			height -= 2
		}
		step.SetSize(s.width, max(1, height))
	}
}

func (s *LoadingStep[T]) CompactHelp() string {
	if !s.ready {
		return s.Help()
	}
	help := s.Step.Help()
	if step, ok := s.Step.(framework.CompactHelper); ok {
		help = step.CompactHelp()
	}
	return help + " • " + framework.BindingHelp(framework.Keys.Refresh)
}
