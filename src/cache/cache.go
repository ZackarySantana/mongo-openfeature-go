package cache

import (
	"fmt"
	"sync"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func New() *Cache {
	return &Cache{
		cacheMutex: sync.RWMutex{},
		cache:      make(map[string]flag.Definition),
	}
}

type Cache struct {
	cacheMutex sync.RWMutex `bson:"-"`
	cache      map[string]flag.Definition
}

func (c *Cache) Clear() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cache = make(map[string]flag.Definition)
}

func (c *Cache) Set(flagKey string, definition any) error {
	parsedDefinition, ok := definition.(flag.Definition)
	if ok {
		c.cacheMutex.Lock()
		defer c.cacheMutex.Unlock()
		c.cache[flagKey] = parsedDefinition
		return nil
	}
	// If the definition is not of type FlagDefinition, we attempt to parse it.
	bsonDefinition, err := bson.Marshal(definition)
	if err != nil {
		return fmt.Errorf("marshalling definition to bson: %w", err)
	}
	if err := bson.Unmarshal(bsonDefinition, &parsedDefinition); err != nil {
		return fmt.Errorf("unmarshalling bson to flag definition: %w", err)
	}

	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cache[flagKey] = parsedDefinition

	return nil
}

func (c *Cache) SetAll(definitions map[string]flag.Definition) error {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	for flagKey, definition := range definitions {
		if err := c.Set(flagKey, definition); err != nil {
			return fmt.Errorf("setting flag %s: %w", flagKey, err)
		}
	}

	return nil
}

func Evaluate[T any](cache *Cache, flatCtx openfeature.FlattenedContext, flag string, defaultValue T) (T, openfeature.ProviderResolutionDetail) {
	cache.cacheMutex.RLock()
	defer cache.cacheMutex.RUnlock()

	flagDefinition, ok := cache.cache[flag]
	if !ok {
		return defaultValue, openfeature.ProviderResolutionDetail{
			Reason: openfeature.DefaultReason,
		}
	}

	val, detail := flagDefinition.Evaluate(flatCtx)

	parsedVal, ok := val.(T)
	if !ok {
		return defaultValue, openfeature.ProviderResolutionDetail{
			Reason:          openfeature.ErrorReason,
			ResolutionError: openfeature.NewTypeMismatchResolutionError(fmt.Sprintf("expected type %T, got %T", defaultValue, val)),
		}
	}

	return parsedVal, detail
}
