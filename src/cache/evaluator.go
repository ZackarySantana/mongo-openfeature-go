package cache

import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
)

func NewEvaluator(cache *Cache) *CacheEvaluator {
	return &CacheEvaluator{
		cache: cache,
	}
}

type CacheEvaluator struct {
	cache *Cache
}

// BooleanEvaluation implements openfeature.FeatureProvider.
func (c *CacheEvaluator) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, flatCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	val, detail := Evaluate(c.cache, flatCtx, flag, defaultValue)
	return openfeature.BoolResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// FloatEvaluation implements openfeature.FeatureProvider.
func (c *CacheEvaluator) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, flatCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	val, detail := Evaluate(c.cache, flatCtx, flag, defaultValue)
	return openfeature.FloatResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// IntEvaluation implements openfeature.FeatureProvider.
func (c *CacheEvaluator) IntEvaluation(ctx context.Context, flag string, defaultValue int64, flatCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	val, detail := Evaluate(c.cache, flatCtx, flag, defaultValue)
	return openfeature.IntResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// ObjectEvaluation implements openfeature.FeatureProvider.
func (c *CacheEvaluator) ObjectEvaluation(ctx context.Context, flag string, defaultValue any, flatCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	val, detail := Evaluate(c.cache, flatCtx, flag, defaultValue)
	return openfeature.InterfaceResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// StringEvaluation implements openfeature.FeatureProvider.
func (c *CacheEvaluator) StringEvaluation(ctx context.Context, flag string, defaultValue string, flatCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	val, detail := Evaluate(c.cache, flatCtx, flag, defaultValue)
	return openfeature.StringResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}
