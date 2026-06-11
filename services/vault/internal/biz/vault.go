package biz

import (
	"application/internal/entity"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// Buyback payout percentages (CLAUDE.md §1). Integer math only; truncation
// toward zero (Go integer division), documented on the buyback path.
const (
	buybackCashPercent   = 50 // CASH: 50% of appraised value, in USDC
	buybackCreditPercent = 85 // CREDIT: 85% of appraised value, as Vault Credit
	percentDivisor       = 100
)

type vault struct {
	logger *slog.Logger
	repo   RepositoryVault
}

var _ UsecaseVault = (*vault)(nil)

// NewVault constructs the vault use case.
func NewVault(logger *slog.Logger, repo RepositoryVault) *vault {
	return &vault{
		logger: logger.With("layer", "VaultUsecase"),
		repo:   repo,
	}
}

// View implements UsecaseVault.
func (uc *vault) View(ctx context.Context, owner uuid.UUID) (VaultView, error) {
	objects, err := uc.repo.ListObjects(ctx, owner)
	if err != nil {
		return VaultView{}, err
	}

	balance, err := uc.repo.CreditBalance(ctx, owner)
	if err != nil {
		return VaultView{}, err
	}

	return VaultView{Objects: objects, CreditBalanceCents: balance}, nil
}

// AddObject implements UsecaseVault.
func (uc *vault) AddObject(
	ctx context.Context,
	owner uuid.UUID,
	title, description string,
	appraisedValueCents int64,
) (entity.VaultObject, error) {
	logger := uc.logger.With("method", "AddObject", "owner", owner)

	if appraisedValueCents <= 0 {
		return entity.VaultObject{}, fmt.Errorf("%w: appraised value must be positive", ErrResourceInvalid)
	}

	obj := entity.VaultObject{
		ID:                  uuid.New(),
		OwnerAccountID:      owner,
		Title:               title,
		Description:         description,
		AppraisedValueCents: appraisedValueCents,
		State:               entity.ObjectInVault,
	}

	created, err := uc.repo.InsertObject(ctx, obj)
	if err != nil {
		return entity.VaultObject{}, err
	}

	logger.InfoContext(ctx, "object added", "object", created.ID)

	return created, nil
}

// List implements UsecaseVault: validate the mode/duration matrix, assert
// ownership, then atomically transition IN_VAULT -> APPRAISING and emit
// object.listed via the outbox.
func (uc *vault) List(
	ctx context.Context,
	owner, objectID uuid.UUID,
	req ListRequest,
) (entity.VaultObject, error) {
	logger := uc.logger.With("method", "List", "owner", owner, "object", objectID)

	if err := validateListRequest(req); err != nil {
		logger.WarnContext(ctx, "invalid list request", "error", err)

		return entity.VaultObject{}, err
	}

	obj, err := uc.repo.GetObject(ctx, objectID)
	if err != nil {
		return entity.VaultObject{}, err
	}

	if obj.OwnerAccountID != owner {
		return entity.VaultObject{}, fmt.Errorf("%w: object not owned by caller", ErrResourceAccessDenied)
	}

	if obj.State != entity.ObjectInVault {
		return entity.VaultObject{}, fmt.Errorf("%w: object not IN_VAULT (state=%s)", ErrResourceInvalid, obj.State)
	}

	// Categorization + ≤7 media + the 4-language content (back-filled from the
	// primary), validated and normalized before persistence.
	details, err := buildListingDetails(req)
	if err != nil {
		logger.WarnContext(ctx, "invalid listing details", "error", err)

		return entity.VaultObject{}, err
	}

	// Producer-stable key for this listing: one logical list per object listing.
	idempotencyKey := fmt.Sprintf("vault:listed:%s", objectID)

	outbox, err := newObjectListedOutbox(objectID, owner, req.Mode, req.DurationDays, obj.AppraisedValueCents, details, idempotencyKey)
	if err != nil {
		return entity.VaultObject{}, err
	}

	// Listed items go to PENDING_INSPECTION: not public until an Inspector seals
	// them (§3.5). The catalog gate independently keeps the lot out of the gallery.
	updated, err := uc.repo.ListWithDetailsTx(ctx, objectID,
		entity.ObjectInVault, entity.ObjectPendingInspection, details, outbox)
	if err != nil {
		return entity.VaultObject{}, err
	}

	logger.InfoContext(ctx, "object listed", "mode", req.Mode, "durationDays", req.DurationDays,
		"category", details.CategoryCode, "images", len(details.ImageRefs))

	return updated, nil
}

// buildListingDetails validates the categorization, media cap, and the 4-language
// content, back-filling any blank language from the primary so every supported
// language is present (CLAUDE.md §0 rule 7). Returns ErrResourceInvalid on a
// missing category, >7 images, or a blank primary title/description.
func buildListingDetails(req ListRequest) (entity.ListingDetails, error) {
	if req.CategoryCode == "" {
		return entity.ListingDetails{}, fmt.Errorf("%w: category is required", ErrResourceInvalid)
	}
	if len(req.ImageRefs) > 7 {
		return entity.ListingDetails{}, fmt.Errorf("%w: at most 7 images per item", ErrResourceInvalid)
	}

	primary := req.PrimaryLang
	if !isSupportedLang(primary) {
		return entity.ListingDetails{}, fmt.Errorf("%w: primaryLang must be one of en/fa/ar/tr", ErrResourceInvalid)
	}

	// Index provided translations by language.
	byLang := make(map[string]entity.ObjectTranslation, len(req.Translations))
	for _, t := range req.Translations {
		if !isSupportedLang(t.Lang) {
			return entity.ListingDetails{}, fmt.Errorf("%w: unsupported language %q", ErrResourceInvalid, t.Lang)
		}
		byLang[t.Lang] = t
	}

	p, ok := byLang[primary]
	if !ok || p.Title == "" || p.Description == "" {
		return entity.ListingDetails{}, fmt.Errorf("%w: primary language title/description required", ErrResourceInvalid)
	}

	// Back-fill every supported language from the primary where blank.
	full := make([]entity.ObjectTranslation, 0, len(entity.SupportedLangs))
	for _, lang := range entity.SupportedLangs {
		t := byLang[lang]
		t.Lang = lang
		if t.Title == "" {
			t.Title = p.Title
		}
		if t.Description == "" {
			t.Description = p.Description
		}
		full = append(full, t)
	}

	return entity.ListingDetails{
		CategoryCode: req.CategoryCode,
		PrimaryLang:  primary,
		Translations: full,
		ImageRefs:    req.ImageRefs,
	}, nil
}

// isSupportedLang reports whether lang is one of the 4 supported languages.
func isSupportedLang(lang string) bool {
	for _, l := range entity.SupportedLangs {
		if l == lang {
			return true
		}
	}

	return false
}

// Buyback implements UsecaseVault: compute the integer payout, transition
// IN_VAULT -> BOUGHT_BACK, and for CREDIT append the ledger row + emit
// credit.changed (all atomic).
func (uc *vault) Buyback(
	ctx context.Context,
	owner, objectID uuid.UUID,
	mode entity.BuybackMode,
) (BuybackResult, error) {
	logger := uc.logger.With("method", "Buyback", "owner", owner, "object", objectID)

	if !mode.Valid() {
		return BuybackResult{}, fmt.Errorf("%w: unknown buyback mode %q", ErrResourceInvalid, mode)
	}

	obj, err := uc.repo.GetObject(ctx, objectID)
	if err != nil {
		return BuybackResult{}, err
	}

	if obj.OwnerAccountID != owner {
		return BuybackResult{}, fmt.Errorf("%w: object not owned by caller", ErrResourceAccessDenied)
	}

	if obj.State != entity.ObjectInVault {
		return BuybackResult{}, fmt.Errorf("%w: buyback only from IN_VAULT (state=%s)", ErrResourceInvalid, obj.State)
	}

	payout := buybackPayoutCents(obj.AppraisedValueCents, mode)

	// CASH settles off the vault ledger (USDC paid out elsewhere): just flip the
	// object state, no ledger row, no event.
	if mode == entity.BuybackModeCash {
		updated, _, txErr := uc.repo.BuybackTx(ctx, objectID, nil, nil)
		if txErr != nil {
			return BuybackResult{}, txErr
		}

		logger.InfoContext(ctx, "cash buyback", "payoutCents", payout)

		return BuybackResult{Object: updated, Mode: mode, PayoutCents: payout}, nil
	}

	// CREDIT: append a signed ledger row and emit credit.changed atomically with
	// the state flip. RefID/idempotency are scoped to the object's buyback. The
	// outbox is built inside the tx from the resulting balance.
	refID := fmt.Sprintf("buyback:%s", objectID)
	idempotencyKey := fmt.Sprintf("vault:credit:buyback:%s", objectID)

	entry := &entity.VaultCreditEntry{
		ID:         uuid.New(),
		AccountID:  owner,
		DeltaCents: payout,
		Reason:     entity.CreditBuyback,
		RefID:      refID,
	}

	buildOutbox := func(balanceCents int64) (entity.OutboxEvent, error) {
		return newCreditChangedOutbox(owner, payout, balanceCents, entity.CreditBuyback, idempotencyKey)
	}

	updated, balance, err := uc.repo.BuybackTx(ctx, objectID, entry, buildOutbox)
	if err != nil {
		return BuybackResult{}, err
	}

	logger.InfoContext(ctx, "credit buyback", "payoutCents", payout, "balanceCents", balance)

	return BuybackResult{Object: updated, Mode: mode, PayoutCents: payout, BalanceCents: balance}, nil
}

// SettleAuctionCompleted implements UsecaseVault: idempotently mark an owned
// IN_AUCTION object SOLD, optionally crediting the seller's Vault-Credit release.
func (uc *vault) SettleAuctionCompleted(ctx context.Context, in AuctionCompletedInput) error {
	logger := uc.logger.With("method", "SettleAuctionCompleted", "object", in.ObjectID)

	obj, err := uc.repo.GetObject(ctx, in.ObjectID)
	if err != nil {
		// Unknown object: nothing this service owns. Treat as a no-op success so
		// a shared stream doesn't redeliver forever.
		if errors.Is(err, ErrResourceNotFound) {
			logger.InfoContext(ctx, "auction.completed for unknown object; ignoring")

			return nil
		}

		return err
	}

	// Already settled (terminal) — idempotent no-op.
	if obj.State == entity.ObjectSold || obj.State == entity.ObjectBoughtBack {
		logger.InfoContext(ctx, "object already terminal; ignoring", "state", obj.State)

		return nil
	}

	var (
		entry       *entity.VaultCreditEntry
		buildOutbox OutboxBuilder
	)

	if in.AsVaultCredit && in.ReleaseCents > 0 {
		refID := fmt.Sprintf("auction-release:%s", in.ObjectID)
		creditKey := fmt.Sprintf("vault:credit:release:%s", in.ObjectID)
		ownerID := obj.OwnerAccountID

		entry = &entity.VaultCreditEntry{
			ID:         uuid.New(),
			AccountID:  ownerID,
			DeltaCents: in.ReleaseCents,
			Reason:     entity.CreditAuctionRelease,
			RefID:      refID,
		}

		buildOutbox = func(balanceCents int64) (entity.OutboxEvent, error) {
			return newCreditChangedOutbox(ownerID, in.ReleaseCents, balanceCents, entity.CreditAuctionRelease, creditKey)
		}
	}

	balance, err := uc.repo.SettleSoldTx(ctx, in.ObjectID, in.IdempotencyKey, entry, buildOutbox)
	if err != nil {
		if errors.Is(err, ErrResourceExists) {
			logger.InfoContext(ctx, "duplicate auction.completed ignored", "key", in.IdempotencyKey)

			return nil
		}

		return err
	}

	logger.InfoContext(ctx, "object settled SOLD", "asVaultCredit", in.AsVaultCredit, "balanceCents", balance)

	return nil
}

// validateListRequest enforces the mode/duration matrix (CLAUDE.md §6):
// DurationDays is REQUIRED (2/5/7) for timed modes and FORBIDDEN for DUTCH.
func validateListRequest(req ListRequest) error {
	if !req.Mode.Valid() {
		return fmt.Errorf("%w: unknown auction mode %q", ErrResourceInvalid, req.Mode)
	}

	if req.Mode.Timed() {
		if !entity.ValidDurationDays(req.DurationDays) {
			return fmt.Errorf("%w: durationDays must be 2/5/7 for %s", ErrResourceInvalid, req.Mode)
		}

		return nil
	}

	// DUTCH: duration forbidden.
	if req.DurationDays != 0 {
		return fmt.Errorf("%w: durationDays forbidden for DUTCH", ErrResourceInvalid)
	}

	return nil
}

// buybackPayoutCents computes the instant-buyback payout in USDC cents using
// integer math (truncation toward zero): CASH = value*50/100, CREDIT = value*85/100.
func buybackPayoutCents(appraisedValueCents int64, mode entity.BuybackMode) int64 {
	switch mode {
	case entity.BuybackModeCash:
		return appraisedValueCents * buybackCashPercent / percentDivisor
	case entity.BuybackModeCredit:
		return appraisedValueCents * buybackCreditPercent / percentDivisor
	default:
		return 0
	}
}
