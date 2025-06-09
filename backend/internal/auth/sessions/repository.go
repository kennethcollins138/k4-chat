package sessions

import "context"

type SessionRepository interface {
	FindByID(ctx context.Context, id string) (*Session, error)
	Insert(ctx context.Context, session *Session) error
	Update(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
	ListByUserID(ctx context.Context, userID string) ([]*Session, error)
}
