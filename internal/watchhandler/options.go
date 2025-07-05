package watchhandler

import (
	"context"
	"log/slog"

	"github.com/zackarysantana/mongo-openfeature-go/internal/eventhandler"
	mongoopenfeature "github.com/zackarysantana/mongo-openfeature-go/src"
	"go.mongodb.org/mongo-driver/v2/mongo"
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
	// Logger is the logger to use for the watch handler.
	// This is only used for logging service-fatal errors.
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

func NewOptions(client *mongo.Client, database, collection string) *Options {
	return &Options{
		Client:     client,
		Database:   database,
		Collection: collection,
	}
}

func (opts *Options) WithEventHandler(eventHandler *eventhandler.EventHandler) *Options {
	if opts == nil {
		opts = &Options{}
	}
	opts.EventHandler = eventHandler
	return opts
}

func (opts *Options) WithLogger(logger *slog.Logger) *Options {
	if opts == nil {
		opts = &Options{}
	}
	opts.Logger = logger
	return opts
}

func (opts *Options) WithDocumentID(documentID string) *Options {
	if opts == nil {
		opts = &Options{}
	}
	opts.DocumentID = documentID
	return opts
}

func (opts *Options) WithMaxTries(maxTries int) *Options {
	if opts == nil {
		opts = &Options{}
	}
	opts.MaxTries = maxTries
	return opts
}

func (opts *Options) WithParentContext(ctx context.Context) *Options {
	if opts == nil {
		opts = &Options{}
	}
	opts.ParentContext = ctx
	return opts
}

func (opts *Options) Validate() error {
	if opts == nil {
		return mongoopenfeature.ErrNilOptions
	}
	// Validating
	if opts.Client == nil {
		return mongoopenfeature.ErrMissingClient
	}
	if opts.Database == "" {
		return mongoopenfeature.ErrMissingDatabase
	}
	if opts.Collection == "" {
		return mongoopenfeature.ErrMissingCollection
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
