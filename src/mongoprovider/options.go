package mongoprovider

import (
	"log/slog"

	mongoopenfeature "github.com/zackarysantana/mongo-openfeature-go/src"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Options struct {
	// ===== Required =====

	// Client is the MongoDB client to use for the provider.
	// This provider does not manage the lifecycle of the client.
	Client *mongo.Client
	// Database is the name of the MongoDB database to use.
	Database string
	// Collection is the name of the MongoDB collection to use.
	Collection string

	// ===== Optional =====

	// DocumentID is the ID of the document to use for feature flags.
	// if none is provided, the provider will use the full collection
	// to evaluate feature flags.
	DocumentID string
	// Logger is the logger to use for the provider.
	// This is only used for logging service-fatal errors.
	Logger *slog.Logger
}

func NewOptions(client *mongo.Client, database, collection, documentID string) *Options {
	return &Options{
		Client:     client,
		Database:   database,
		Collection: collection,
		DocumentID: documentID,
	}
}

func (opts *Options) WithLogger(logger *slog.Logger) *Options {
	if opts == nil {
		opts = &Options{}
	}
	opts.Logger = logger
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
	return nil
}
