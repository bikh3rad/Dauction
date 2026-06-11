package handler

import (
	"application/internal/biz"
	"application/internal/service/dto"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// headerAccountID is the authenticated subject the gateway injects after authN.
// For inspector routes the gateway has already enforced the INSPECTOR role.
const headerAccountID = "X-Account-Id"

// inspectorQueue lists DRAFT lots awaiting an Inspector seal.
//
//	@Summary		Inspector queue
//	@Description	Lots awaiting inspection (DRAFT). Requires the INSPECTOR role (enforced at the gateway). This is the auction-eligibility gate (§3.5).
//	@Tags			catalog-inspector
//	@Produce		json
//	@Success		200	{array}		dto.LotResp
//	@Failure		500	{object}	dto.ErrorResponse
//	@Router			/apis/inspector/queue [get]
func (h *lotHandler) inspectorQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "InspectorQueue")

	lots, err := h.lot.InspectionQueue(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "inspection queue failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToLotResps(lots))
}

// inspect records an Inspector's sealing verdict on a lot.
//
//	@Summary		Inspect a lot
//	@Description	Inspector seals a lot: APPROVED certifies it (DRAFT->CERTIFIED) and lets it reach the gallery; REJECTED blocks it. Requires GENUINE + a condition grade to approve. Emits attestation.recorded (+ lot.certified on approve).
//	@Tags			catalog-inspector
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Lot UUID"
//	@Param			body	body		dto.InspectRequest	true	"Sealing verdict"
//	@Success		200		{object}	dto.LotResp
//	@Failure		400		{object}	dto.ErrorResponse	"Bad request / illegal state"
//	@Failure		404		{object}	dto.ErrorResponse	"Lot not found"
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/apis/inspector/lots/{id}/inspect [post]
func (h *lotHandler) inspect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "Inspect")

	lotID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	inspectorID, err := uuid.Parse(r.Header.Get(headerAccountID))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, err), w)

		return
	}

	var req dto.InspectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	verdict, err := req.Validate()
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	l, err := h.lot.Inspect(ctx, lotID, biz.InspectInput{
		InspectorID:    inspectorID,
		Verdict:        verdict,
		Authenticity:   req.Authenticity,
		ConditionGrade: req.ConditionGrade,
		Notes:          req.Notes,
	})
	if err != nil {
		logger.WarnContext(ctx, "inspect failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToLotResp(l))
}

// categories returns the catalog's category vocabulary (public). Codes are
// language-neutral; the client localizes display names and maps icon_key to the
// per-category icon set (the design directive).
//
//	@Summary		Categories
//	@Description	The catalog category vocabulary (code + icon_key). Public; the client localizes labels.
//	@Tags			catalog
//	@Produce		json
//	@Success		200	{array}	dto.CategoryResp
//	@Router			/apis/categories [get]
func (h *lotHandler) categories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	writeJSON(ctx, w, h.logger, http.StatusOK, dto.SeededCategories())
}
