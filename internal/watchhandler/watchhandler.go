package watchhandler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/eventhandler"
	"github.com/zackarysantana/mongo-openfeature-go/src/cache"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func New(opts *Options) (*WatchHandler, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("validating watch options: %w", err)
	}
	ctx, cancel := context.WithCancelCause(opts.ParentContext)
	return &WatchHandler{
		ctx:    ctx,
		cancel: cancel,

		collection: opts.Client.Database(opts.Database).Collection(opts.Collection),
		maxTries:   opts.MaxTries,
		documentID: opts.DocumentID,

		eventHandler: opts.EventHandler,
		cache:        opts.Cache,
		logger:       opts.Logger,
	}, nil
}

type WatchHandler struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	collection *mongo.Collection
	maxTries   int
	documentID string

	eventHandler *eventhandler.EventHandler
	cache        *cache.Cache
	logger       *slog.Logger
}

func (w *WatchHandler) Watch() {
	success := false
	for attempt := 0; attempt <= w.maxTries; attempt++ {
		err := w.baseWatch()
		if err == nil {
			success = true
			break
		}
		w.logger.Error("error in watch", "error", err, "attempt", attempt, "documentID", w.documentID)
	}

	if !success {
		w.logger.Error("max retries reached, stopping watch", "tries", w.maxTries, "documentID", w.documentID)
		if w.eventHandler != nil {
			w.eventHandler.BPublish(openfeature.Event{
				ProviderName: "WatchHandler",
				EventType:    openfeature.ProviderError,
				ProviderEventDetails: openfeature.ProviderEventDetails{
					Message:   fmt.Sprintf("Max retries reached (%d). Stopping watch.", w.maxTries),
					ErrorCode: openfeature.ProviderFatalCode,
				},
			})
		}
		return
	}
}

func (w *WatchHandler) baseWatch() error {
	var pipeline mongo.Pipeline
	if w.documentID != "" {
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"operationType":    bson.M{"$in": []string{"insert", "update ", "replace"}},
				"fullDocument._id": w.documentID,
			}}},
		}
	} else {
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"operationType": bson.M{"$in": []string{"insert", "update", "replace"}},
			}}},
		}
	}
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	ctx, cancel := context.WithCancel(w.ctx)
	defer cancel()

	cs, err := w.collection.Watch(ctx, pipeline, opts)
	if err != nil {
		return fmt.Errorf("starting change stream: %w", err)
	}
	defer cs.Close(context.WithoutCancel(w.ctx))

	for cs.Next(ctx) {
		if err := cs.Err(); err != nil {
			return fmt.Errorf("change stream error: %w", err)
		}
		var csEvent ChangeStreamEvent
		if err := cs.Decode(&csEvent); err != nil {
			return fmt.Errorf("decoding change stream document: %w", err)
		}
		if err := w.handleEvent(csEvent); err != nil {
			return fmt.Errorf("handling change stream event: %w", err)
		}
	}
	if err := cs.Err(); err != nil {
		return fmt.Errorf("error iterating change stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		if err == context.Canceled {
			w.logger.Info("watch cancelled", "documentID", w.documentID)
			return nil
		}
		return fmt.Errorf("context error: %w", err)
	}
	return nil
}

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

func (w *WatchHandler) Close() {
	if w.cancel != nil {
		w.cancel(context.Canceled)
	}
}
