package apictx

import (
	"context"

	"probakgo/internal/domain"
)

type contextKey string

const apiKeyContextKey contextKey = "api_key"

func WithAPIKey(ctx context.Context, key *domain.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyContextKey, key)
}

func APIKey(ctx context.Context) (*domain.APIKey, bool) {
	key, ok := ctx.Value(apiKeyContextKey).(*domain.APIKey)
	return key, ok && key != nil
}
