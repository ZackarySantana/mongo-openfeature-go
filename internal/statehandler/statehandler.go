package statehandler

import (
	"errors"

	"github.com/open-feature/go-sdk/openfeature"
)

var _ openfeature.StateHandler = (*StateHandler)(nil)

func New() *StateHandler {
	return &StateHandler{
		status: openfeature.NotReadyState,
	}
}

type StateHandler struct {
	status   openfeature.State
	startup  []func() error
	shutdown []func()
}

func (s *StateHandler) Init(evaluationContext openfeature.EvaluationContext) error {
	if s.status != openfeature.NotReadyState {
		return errors.New("state handler is already initialized")
	}
	for _, fn := range s.startup {
		if err := fn(); err != nil {
			return err
		}
	}
	s.startup = nil

	s.status = openfeature.ReadyState
	return nil
}

// RegisterStartupFunc registers a function to be called when the state handler is initialized.
func (s *StateHandler) RegisterStartupFunc(fn func() error) {
	s.startup = append(s.startup, fn)
}

// RegisterShutdownFunc registers a function to be called when the state handler is shut down.
func (s *StateHandler) RegisterShutdownFunc(fn func()) {
	s.shutdown = append(s.shutdown, fn)
}

// Shutdown calls all registered shutdown functions in the order they were registered.
func (s *StateHandler) Shutdown() {
	for _, fn := range s.shutdown {
		fn()
	}
	s.shutdown = nil // Clear the shutdown functions after calling them
}
