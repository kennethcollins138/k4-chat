package tokens

import "context"

type TokenRepository interface {
	Store(ctx context.Context, token string, claims *TokenClaims) error
	Find(ctx context.Context, token string) (*TokenClaims, error)
	Delete(ctx context.Context, token string) error
	Blacklist(ctx context.Context, token string) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}
