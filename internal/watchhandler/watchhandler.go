package watchhandler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/eventhandler"
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
	logger       *slog.Logger
}

func (w *WatchHandler) Watch(callback func(ChangeStreamEvent) error) {
	success := false
	for attempt := 0; attempt <= w.maxTries; attempt++ {
		err := w.baseWatch(func(event ChangeStreamEvent) error {
			w.eventHandler.Publish(openfeature.Event{
				ProviderName: "WatchHandler",
				EventType:    openfeature.ProviderConfigChange,
			})

			return callback(event)
		})
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

func (w *WatchHandler) baseWatch(callback func(ChangeStreamEvent) error) error {
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
		if err := callback(csEvent); err != nil {
			return fmt.Errorf("callback error: %w", err)
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

func (w *WatchHandler) Close() {
	if w.cancel != nil {
		w.cancel(context.Canceled)
	}
}
