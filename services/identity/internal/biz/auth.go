package biz

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	otpTTL      = 5 * time.Minute
	otpDigits   = 6
	validOAuth  = "GOOGLE FACEBOOK APPLE"
	validPurpos = "SIGNUP LOGIN"
)

type auth struct {
	logger  *slog.Logger
	repo    RepositoryAuth
	devMode bool // dev: return + log OTP codes (no SMS provider)
}

var _ UsecaseAuth = (*auth)(nil)

// NewAuth constructs the onboarding use case. devMode defaults to true for local
// builds (the codes are echoed/logged); production wiring sets it false and
// integrates an SMS provider.
func NewAuth(logger *slog.Logger, repo RepositoryAuth) *auth {
	return &auth{
		logger:  logger.With("layer", "AuthUsecase"),
		repo:    repo,
		devMode: true,
	}
}

// RequestOTP issues a hashed one-time code for a mobile number.
func (uc *auth) RequestOTP(ctx context.Context, mobile, purpose string) (int, string, error) {
	logger := uc.logger.With("method", "RequestOTP")

	mobile = strings.TrimSpace(mobile)
	if !validE164(mobile) {
		return 0, "", fmt.Errorf("%w: invalid mobile number", ErrResourceInvalid)
	}
	purpose = strings.ToUpper(strings.TrimSpace(purpose))
	if purpose == "" {
		purpose = "SIGNUP"
	}
	if !strings.Contains(validPurpos, purpose) {
		return 0, "", fmt.Errorf("%w: invalid purpose", ErrResourceInvalid)
	}

	code, err := randomCode(otpDigits)
	if err != nil {
		return 0, "", err
	}

	if err := uc.repo.InsertOTP(ctx, mobile, hashCode(mobile, code), purpose, time.Now().Add(otpTTL)); err != nil {
		return 0, "", err
	}

	// Dev: log + echo the code (no SMS). Production MUST disable this.
	logger.InfoContext(ctx, "OTP issued", "mobile", mobile, "purpose", purpose, "devCode", code)
	devCode := ""
	if uc.devMode {
		devCode = code
	}

	return int(otpTTL.Seconds()), devCode, nil
}

// VerifyOTP validates a code and returns a session, creating the account on first
// successful verify.
func (uc *auth) VerifyOTP(ctx context.Context, mobile, code string) (AuthResult, error) {
	logger := uc.logger.With("method", "VerifyOTP")

	mobile = strings.TrimSpace(mobile)
	if !validE164(mobile) || strings.TrimSpace(code) == "" {
		return AuthResult{}, fmt.Errorf("%w: missing mobile/code", ErrResourceInvalid)
	}

	ok, err := uc.repo.ConsumeOTP(ctx, mobile, hashCode(mobile, code))
	if err != nil {
		return AuthResult{}, err
	}
	if !ok {
		return AuthResult{}, fmt.Errorf("%w: invalid or expired code", ErrResourceAccessDenied)
	}

	if acc, found, err := uc.repo.FindAccountByMobile(ctx, mobile); err != nil {
		return AuthResult{}, err
	} else if found {
		return AuthResult{Account: acc, Token: acc.ID.String(), Created: false}, nil
	}

	id := uuid.New()
	outbox, err := newRegisteredOutbox(id, mobile, "")
	if err != nil {
		return AuthResult{}, err
	}
	acc, err := uc.repo.CreateMobileAccountTx(ctx, id, mobile, outbox)
	if err != nil {
		return AuthResult{}, err
	}

	logger.InfoContext(ctx, "account registered via mobile", "account", id)

	return AuthResult{Account: acc, Token: acc.ID.String(), Created: true}, nil
}

// OAuthLogin links/creates an account from a social provider. The provider token
// exchange is stubbed in dev: the `code` is treated as the provider user id.
func (uc *auth) OAuthLogin(ctx context.Context, provider, code string) (AuthResult, error) {
	logger := uc.logger.With("method", "OAuthLogin")

	provider = strings.ToUpper(strings.TrimSpace(provider))
	if !strings.Contains(validOAuth, provider) {
		return AuthResult{}, fmt.Errorf("%w: unknown provider", ErrResourceInvalid)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return AuthResult{}, fmt.Errorf("%w: missing oauth code", ErrResourceInvalid)
	}

	// Dev stub for the provider exchange. Production replaces this with a real
	// token exchange + userinfo call to the provider.
	providerUserID := code
	email := ""

	id := uuid.New()
	outbox, err := newRegisteredOutbox(id, "", provider)
	if err != nil {
		return AuthResult{}, err
	}

	acc, created, err := uc.repo.UpsertOAuthTx(ctx, provider, providerUserID, email, id, outbox)
	if err != nil {
		return AuthResult{}, err
	}

	logger.InfoContext(ctx, "oauth login", "account", acc.ID, "provider", provider, "created", created)

	return AuthResult{Account: acc, Token: acc.ID.String(), Created: created}, nil
}

// --- helpers ---

// randomCode returns an n-digit numeric code using crypto/rand.
func randomCode(n int) (string, error) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteString(d.String())
	}

	return b.String(), nil
}

// hashCode binds the code to the mobile so a hash can't be replayed for another
// number. Not a password — short-lived single-use; sha256 is sufficient.
func hashCode(mobile, code string) string {
	sum := sha256.Sum256([]byte(mobile + ":" + code))

	return hex.EncodeToString(sum[:])
}

// validE164 is a permissive E.164 check: leading '+', 8-15 digits.
func validE164(s string) bool {
	if len(s) < 9 || len(s) > 16 || s[0] != '+' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
