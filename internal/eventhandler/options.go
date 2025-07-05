package eventhandler

import (
	"github.com/open-feature/go-sdk/openfeature"
	mongoopenfeature "github.com/zackarysantana/mongo-openfeature-go/src"
)

type DroppedEventHandler func(event openfeature.Event)

type Options struct {
	// ===== Required =====

	// DroppedEventHandler is the function to call when an event is dropped.
	DroppedEventHandler DroppedEventHandler
}

func NewOptions(droppedEventHandler DroppedEventHandler) *Options {
	return &Options{
		DroppedEventHandler: droppedEventHandler,
	}
}

func (opts *Options) Validate() error {
	if opts == nil {
		return mongoopenfeature.ErrNilOptions
	}
	if opts.DroppedEventHandler == nil {
		return mongoopenfeature.ErrNilDroppedEventHandler
	}
	return nil
}
