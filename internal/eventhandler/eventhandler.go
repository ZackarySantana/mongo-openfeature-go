package eventhandler

import (
	"fmt"

	"github.com/open-feature/go-sdk/openfeature"
)

var _ openfeature.EventHandler = (*EventHandler)(nil)

const (
	eventChannelSize = 10
)

func New(opts *Options) (*EventHandler, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("validating event options: %w", err)
	}
	return &EventHandler{
		eventCh:             make(chan openfeature.Event, eventChannelSize),
		droppedEventHandler: opts.DroppedEventHandler,
	}, nil
}

type EventHandler struct {
	eventCh             chan openfeature.Event
	droppedEventHandler DroppedEventHandler
}

func (h *EventHandler) EventChannel() <-chan openfeature.Event {
	return h.eventCh
}

// Publish publishes an event to the event channel.
// If the channel is full, the
func (h *EventHandler) Publish(event openfeature.Event) {
	select {
	case h.eventCh <- event:
	default:
		h.droppedEventHandler(event)
	}
}

// BPublish blocks until the event is published to the channel.
func (h *EventHandler) BPublish(event openfeature.Event) {
	h.eventCh <- event
}

func (h *EventHandler) Close() {
	close(h.eventCh)
}
