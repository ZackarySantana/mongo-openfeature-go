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

type ChangeStreamEvent struct {
	FullDocument  bson.M `bson:"fullDocument"`
	OperationType string `bson:"operationType"`
}

func (w *WatchHandler) Watch() {
	success := false
	for attempt := 1; attempt <= w.maxTries; attempt++ {
		err := w.changestream()
		if err == nil {
			success = true
			break
		}
		w.logger.Error("error in changestream watching", "error", err, "attempt", attempt, "documentID", w.documentID)
	}
	if success {
		return
	}
	w.logger.Error("change stream failed, falling back to polling", "documentID", w.documentID)

	// Fallback to polling
	for attempt := 1; attempt <= w.maxTries; attempt++ {
		err := w.polling()
		if err == nil {
			success = true
			break
		}
		w.logger.Error("error in polling watching", "error", err, "attempt", attempt, "documentID", w.documentID)
	}
	if success {
		return
	}

	w.logger.Error("max retries reached, stopping watch", "tries", w.maxTries, "documentID", w.documentID)
	if w.eventHandler != nil {
		w.eventHandler.BPublish(openfeature.Event{
			ProviderName: "WatchHandler",
			EventType:    openfeature.ProviderError,
			ProviderEventDetails: openfeature.ProviderEventDetails{
				Message:   fmt.Sprintf("Max retries reached with change streams and polling (%d). Stopping watch.", w.maxTries),
				ErrorCode: openfeature.ProviderFatalCode,
			},
		})
	}
}

func (w *WatchHandler) Close() {
	if w.cancel != nil {
		w.cancel(context.Canceled)
	}
}
