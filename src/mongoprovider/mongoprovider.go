package mongoprovider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/eventhandler"
	"github.com/zackarysantana/mongo-openfeature-go/internal/statehandler"
	"github.com/zackarysantana/mongo-openfeature-go/internal/watchhandler"
	"github.com/zackarysantana/mongo-openfeature-go/src/cache"
	"github.com/zackarysantana/mongo-openfeature-go/src/client"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	ProviderName = "MongoDBFeatureProvider"
)

var _ openfeature.FeatureProvider = (*Provider)(nil)
var _ openfeature.EventHandler = (*Provider)(nil)
var _ openfeature.StateHandler = (*Provider)(nil)

func New(opts *Options) (*Provider, *client.Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validating options: %w", err)
	}
	cacheHandler := cache.New()

	eventHandler, err := eventhandler.New(eventhandler.NewOptions(
		eventhandler.CreateDroppedEventLogger(opts.Logger, ProviderName),
	))
	if err != nil {
		return nil, nil, fmt.Errorf("creating event handler: %w", err)
	}
	watchHandler, err := watchhandler.New(watchhandler.NewOptions(opts.Client, opts.Database, opts.Collection, cacheHandler).
		WithEventHandler(eventHandler).
		WithDocumentID(opts.DocumentID).
		WithLogger(opts.Logger),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating watch handler: %w", err)
	}

	client, err := client.New(client.NewOptions(opts.Client, opts.Database, opts.Collection).
		WithDocumentID(opts.DocumentID).
		WithLogger(opts.Logger),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("creating mongo openfeature client: %w", err)
	}

	p := &Provider{
		EventHandler:   eventHandler,
		StateHandler:   statehandler.New(),
		CacheEvaluator: cache.NewEvaluator(cacheHandler),
		cache:          cacheHandler,
		logger:         opts.Logger,
	}
	p.StateHandler.RegisterShutdownFunc(p.EventHandler.Close)

	p.StateHandler.RegisterStartupFunc(func() error {
		go watchHandler.Watch()
		return nil
	})
	p.StateHandler.RegisterShutdownFunc(watchHandler.Close)

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
	return p, client, nil
}

type Provider struct {
	*eventhandler.EventHandler
	*statehandler.StateHandler
	*cache.CacheEvaluator
	cache *cache.Cache

	logger *slog.Logger
}

func (s *Provider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: ProviderName}
}

func (s *Provider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}
