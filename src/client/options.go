package client

import (
	"log/slog"

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

	// Logger is the logger to use for the watch handler.
	// This is only used for logging service-fatal errors.
	Logger *slog.Logger
	// DocumentID is the ID of the document to query for.
	// This is optional, and if not provided, the query will
	// target all documents in the collection.
	DocumentID string
	// MaxTries is the maximum number of tries to attempt
	// queries. If not provided, it defaults to 2.
	MaxTries int
}

func NewOptions(client *mongo.Client, database, collection string) *Options {
	return &Options{
		Client:     client,
		Database:   database,
		Collection: collection,
	}
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
		opts.MaxTries = 2
	}
	return nil
}
