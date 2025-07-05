package singledocument

import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/zackarysantana/mongo-openfeature-go/internal/cache"
)

// BooleanEvaluation implements openfeature.FeatureProvider.
func (s *SingleDocumentProvider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, flatCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	val, detail := cache.Evaluate(s.cache, flatCtx, flag, defaultValue)
	return openfeature.BoolResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// FloatEvaluation implements openfeature.FeatureProvider.
func (s *SingleDocumentProvider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, flatCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	val, detail := cache.Evaluate(s.cache, flatCtx, flag, defaultValue)
	return openfeature.FloatResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// IntEvaluation implements openfeature.FeatureProvider.
func (s *SingleDocumentProvider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, flatCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	val, detail := cache.Evaluate(s.cache, flatCtx, flag, defaultValue)
	return openfeature.IntResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// ObjectEvaluation implements openfeature.FeatureProvider.
func (s *SingleDocumentProvider) ObjectEvaluation(ctx context.Context, flag string, defaultValue any, flatCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	val, detail := cache.Evaluate(s.cache, flatCtx, flag, defaultValue)
	return openfeature.InterfaceResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}

// StringEvaluation implements openfeature.FeatureProvider.
func (s *SingleDocumentProvider) StringEvaluation(ctx context.Context, flag string, defaultValue string, flatCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	val, detail := cache.Evaluate(s.cache, flatCtx, flag, defaultValue)
	return openfeature.StringResolutionDetail{Value: val, ProviderResolutionDetail: detail}
}
