package cache

import (
	"github.com/open-feature/go-sdk/openfeature"
)

var (
	ProviderName = "CacheProvider"
)

var _ openfeature.FeatureProvider = (*CacheProvider)(nil)

func NewProvider(c *Cache) *CacheProvider {
	return &CacheProvider{
		CacheEvaluator: NewEvaluator(c),
		cache:          c,
	}
}

type CacheProvider struct {
	*CacheEvaluator
	cache *Cache
}

func (c *CacheProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: ProviderName}
}

func (c *CacheProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}
