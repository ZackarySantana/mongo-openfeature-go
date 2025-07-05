package watchhandler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/zackarysantana/mongo-openfeature-go/internal/eventhandler"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrNilOptions        = errors.New("options cannot be nil")
	ErrMissingClient     = errors.New("missing mongo client")
	ErrMissingDatabase   = errors.New("missing database name")
	ErrMissingCollection = errors.New("missing collection name")
	ErrMissingDocumentID = errors.New("missing document ID")
)

type Options struct {
	// ===== Required =====
	// Client is the MongoDB client to use for the watch handler.
	Client *mongo.Client
	// Database is the name of the MongoDB database to use.
	Database string
	// Collection is the name of the MongoDB collection to use.
	Collection string

	// ===== Optional ======
	// EventHandler is the event handler to use for processing
	// events from the watch.
	EventHandler *eventhandler.EventHandler
	// Logger is a supplementary logger to use for logging events
	// and errors. If not provided, the watch handler will log
	// events to slog.Default().
	Logger *slog.Logger
	// DocumentID is the ID of the document to watch for changes.
	// This is optional, and if not provided, the watch will
	// monitor all changes in the collection.
	DocumentID string
	// MaxTries is the maximum number of tries to attempt
	// when watching for changes. If not provided, it defaults to 3.
	MaxTries int
	// ParentContext is the parent context to use for the watch handler.
	// If not provided, it defaults to context.Background().
	ParentContext context.Context
}

func (opts *Options) Validate() error {
	if opts == nil {
		return ErrNilOptions
	}
	// Validating
	if opts.Client == nil {
		return ErrMissingClient
	}
	if opts.Database == "" {
		return ErrMissingDatabase
	}
	if opts.Collection == "" {
		return ErrMissingCollection
	}

	// Setting defaults
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.MaxTries <= 0 {
		opts.MaxTries = 3
	}
	if opts.ParentContext == nil {
		opts.ParentContext = context.Background()
	}
	return nil
}
