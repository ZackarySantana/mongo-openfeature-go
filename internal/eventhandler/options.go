package eventhandler

import (
	"errors"

	"github.com/open-feature/go-sdk/openfeature"
)

var (
	ErrNilOptions             = errors.New("options cannot be nil")
	ErrNilDroppedEventHandler = errors.New("missing dropped event handler")
)

type DroppedEventHandler func(event openfeature.Event)

type Options struct {
	DroppedEventHandler DroppedEventHandler
}

func (opts *Options) Validate() error {
	if opts == nil {
		return ErrNilOptions
	}
	if opts.DroppedEventHandler == nil {
		return ErrNilDroppedEventHandler
	}
	return nil
}
