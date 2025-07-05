package mongoopenfeature

import "errors"

var (
	ErrNilOptions             = errors.New("options cannot be nil")
	ErrMissingClient          = errors.New("missing mongo client")
	ErrMissingDatabase        = errors.New("missing database name")
	ErrMissingCollection      = errors.New("missing collection name")
	ErrMissingCache           = errors.New("missing cache")
	ErrMissingDocumentID      = errors.New("missing document ID")
	ErrNilDroppedEventHandler = errors.New("missing dropped event handler")
)
