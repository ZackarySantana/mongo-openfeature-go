package singledocument

import (
	"errors"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	ErrNilOptions        = errors.New("options cannot be nil")
	ErrMissingClient     = errors.New("missing mongo client")
	ErrMissingDatabase   = errors.New("missing database name")
	ErrMissingCollection = errors.New("missing collection name")
	ErrMissingDocumentID = errors.New("missing document ID")
)

// TODO: Make a func-opts style constructor/builder for the provider.
type SingleDocumentProviderOptions struct {
	// ===== Required =====
	// Client is the MongoDB client to use for the provider.
	// This provider does not manage the lifecycle of the client.
	Client *mongo.Client
	// Database is the name of the MongoDB database to use.
	Database string
	// Collection is the name of the MongoDB collection to use.
	Collection string
	// DocumentID is the ID of the document to use for feature flags.
	DocumentID string

	// ===== Optional =====
	Logger *slog.Logger
}

func (opts *SingleDocumentProviderOptions) Validate() error {
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
	if opts.DocumentID == "" {
		return ErrMissingDocumentID
	}

	// Setting defaults
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return nil
}
