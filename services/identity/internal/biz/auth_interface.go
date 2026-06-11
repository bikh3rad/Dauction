package biz

import (
	"application/internal/entity"
	"context"
	"time"

	"github.com/google/uuid"
)

// AuthResult is the outcome of a successful sign-in/up. Token is the session
// bearer (dev scheme: the account UUID, matching the gateway's dev auth; prod:
// a real signed JWT). Created reports whether this call provisioned the account.
type AuthResult struct {
	Account entity.Account
	Token   string
	Created bool
}

// UsecaseAuth owns onboarding: mobile-number + OTP and social OAuth. It replaces
// the removed invite-redemption flow (CLAUDE.md §1). On first sign-in it creates
// the account and emits account.registered (which provisions the user's Vault).
type UsecaseAuth interface {
	// RequestOTP issues a one-time code for a mobile number. In dev mode the code
	// is returned (devCode) and logged; in production it is delivered by SMS and
	// devCode is empty. purpose is SIGNUP or LOGIN.
	RequestOTP(ctx context.Context, mobile, purpose string) (expiresInSecs int, devCode string, err error)
	// VerifyOTP validates a code, creating the account on first verify, and
	// returns a session.
	VerifyOTP(ctx context.Context, mobile, code string) (AuthResult, error)
	// OAuthLogin exchanges a provider authorization code (dev: the code is treated
	// as the provider user id) and links/creates the account.
	OAuthLogin(ctx context.Context, provider, code string) (AuthResult, error)
}

// RepositoryAuth is the persistence seam for onboarding (implemented by repo).
type RepositoryAuth interface {
	InsertOTP(ctx context.Context, mobile, codeHash, purpose string, expiresAt time.Time) error
	ConsumeOTP(ctx context.Context, mobile, codeHash string) (bool, error)
	FindAccountByMobile(ctx context.Context, mobile string) (entity.Account, bool, error)
	CreateMobileAccountTx(ctx context.Context, id uuid.UUID, mobile string, outbox entity.OutboxEvent) (entity.Account, error)
	UpsertOAuthTx(ctx context.Context, provider, providerUserID, email string, newID uuid.UUID, outbox entity.OutboxEvent) (entity.Account, bool, error)
}
