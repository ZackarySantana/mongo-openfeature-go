package singledocument

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/cache"
	"github.com/zackarysantana/mongo-openfeature-go/internal/eventhandler"
	"github.com/zackarysantana/mongo-openfeature-go/internal/statehandler"
	"github.com/zackarysantana/mongo-openfeature-go/internal/watchhandler"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	ProviderName = "MongoDBSingleDocumentFeatureProvider"
)

var _ openfeature.FeatureProvider = (*SingleDocumentProvider)(nil)
var _ openfeature.EventHandler = (*SingleDocumentProvider)(nil)
var _ openfeature.StateHandler = (*SingleDocumentProvider)(nil)

// The client's shutdown is expected to be handled by the caller.
func NewProvider(opts *Options) (*SingleDocumentProvider, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("validating options: %w", err)
	}

	eventHandler, err := eventhandler.New(eventhandler.NewOptions(
		eventhandler.CreateDroppedEventLogger(opts.Logger, ProviderName),
	))
	if err != nil {
		return nil, fmt.Errorf("creating event handler: %w", err)
	}
	watchHandler, err := watchhandler.New(watchhandler.NewOptions(opts.Client, opts.Database, opts.Collection).
		WithEventHandler(eventHandler).
		WithLogger(opts.Logger).
		WithDocumentID(opts.DocumentID),
	)
	if err != nil {
		return nil, fmt.Errorf("creating watch handler: %w", err)
	}

	client, err := client.New(client.NewOptions(opts.Client, opts.Database, opts.Collection).
		WithDocumentID(opts.DocumentID).
		WithLogger(opts.Logger),
	)
	if err != nil {
		return nil, fmt.Errorf("creating mongo openfeature client: %w", err)
	}

	p := &SingleDocumentProvider{
		EventHandler:      eventHandler,
		StateHandler:      statehandler.New(),
		watchHandler:      watchHandler,
		openfeatureClient: client,
		collection:        opts.Client.Database(opts.Database).Collection(opts.Collection),
		documentID:        opts.DocumentID,
		cache:             cache.NewCache(),
		logger:            opts.Logger,
	}
	p.StateHandler.RegisterShutdownFunc(p.EventHandler.Close)

	p.StateHandler.RegisterStartupFunc(func() error {
		go p.watchHandler.Watch(func(event watchhandler.ChangeStreamEvent) error {
			if id, ok := event.FullDocument["_id"]; !ok || id != p.documentID {
				return fmt.Errorf("change document ID does not match expected ID: %v != %v", id, p.documentID)
			}
			delete(event.FullDocument, "_id")
			p.cache.Clear()

			for key, value := range event.FullDocument {
				if err := p.cache.Set(key, value); err != nil {
					return fmt.Errorf("setting cache value for key %s: %w", key, err)
				}
			}

			return nil
		})
		return nil
	})
	p.StateHandler.RegisterShutdownFunc(p.watchHandler.Close)

	p.StateHandler.RegisterStartupFunc(func() error {
		// TODO: Edit all contexts to use a timeout and add it to the options.
		flags, err := client.GetAllFlags(context.Background())
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				fmt.Println("No flags found in the document, initializing cache with empty values.")
				return nil
			}
			return fmt.Errorf("getting all flags: %w", err)
		}
		if err := p.cache.SetAll(flags); err != nil {
			return fmt.Errorf("setting all flags in cache: %w", err)
		}

		return nil
	})
	return p, nil
}

type SingleDocumentProvider struct {
	*eventhandler.EventHandler
	*statehandler.StateHandler
	watchHandler      *watchhandler.WatchHandler
	openfeatureClient *client.Client
	cache             *cache.Cache

	collection *mongo.Collection
	documentID string

	logger *slog.Logger
}

func (s *SingleDocumentProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: ProviderName}
}

func (s *SingleDocumentProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}
