package watchhandler

import (
	"fmt"

	"github.com/open-feature/go-sdk/openfeature"
)

func (w *WatchHandler) handleEvent(event ChangeStreamEvent) error {
	if w.eventHandler != nil {
		w.eventHandler.Publish(openfeature.Event{
			ProviderName: "WatchHandler",
			EventType:    openfeature.ProviderConfigChange,
			ProviderEventDetails: openfeature.ProviderEventDetails{
				Message: fmt.Sprintf("Change detected for document ID %s", w.documentID),
			},
		})
	}
	if w.documentID != "" {
		return w.handleEventSingleDocument(event)
	}
	return w.handleEventAllDocuments(event)
}

func (w *WatchHandler) handleEventSingleDocument(event ChangeStreamEvent) error {
	if id, ok := event.FullDocument["_id"]; !ok || id != w.documentID {
		return fmt.Errorf("change document ID does not match expected ID: %v != %v", id, w.documentID)
	}
	delete(event.FullDocument, "_id")
	w.cache.Clear()

	for key, value := range event.FullDocument {
		if err := w.cache.Set(key, value); err != nil {
			return fmt.Errorf("setting cache value for key %s: %w", key, err)
		}
	}

	return nil
}

func (w *WatchHandler) handleEventAllDocuments(event ChangeStreamEvent) error {
	if event.FullDocument == nil {
		return fmt.Errorf("change event does not contain full document")
	}

	id, ok := event.FullDocument["_id"]
	if !ok {
		return fmt.Errorf("change event does not contain document ID")
	}
	idString, ok := id.(string)
	if !ok {
		return fmt.Errorf("document ID is not a string: %v", id)
	}

	delete(event.FullDocument, "_id")
	if err := w.cache.Set(idString, event.FullDocument); err != nil {
		return fmt.Errorf("setting cache value for document ID %s: %w", idString, err)
	}

	return nil
}
