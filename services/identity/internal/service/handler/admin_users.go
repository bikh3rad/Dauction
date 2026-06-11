package handler

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/service/dto"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// listUsers returns admin-filtered accounts.
//
//	@Summary		List users (admin)
//	@Description	Admin user search/listing with optional status/role/free-text filters and pagination.
//	@Tags			identity-admin
//	@Produce		json
//	@Param			status	query		string	false	"Status filter (REGISTERED|ACTIVE|SUSPENDED|BANNED)"
//	@Param			role	query		string	false	"Role filter (INSPECTOR|ADMIN)"
//	@Param			q		query		string	false	"Free-text over handle/mobile"
//	@Param			limit	query		int		false	"Page size (default 50, max 200)"
//	@Param			offset	query		int		false	"Page offset"
//	@Success		200		{object}	dto.ListUsersResp
//	@Failure		500		{object}	dto.ErrorResponse
//	@Router			/apis/admin/users [get]
func (h *accountHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "ListUsers")

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	users, total, err := h.account.ListUsers(ctx, biz.UserFilter{
		Status: q.Get("status"),
		Role:   q.Get("role"),
		Query:  q.Get("q"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		logger.ErrorContext(ctx, "list users failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	resp := dto.ListUsersResp{Users: make([]dto.AccountResp, 0, len(users)), Total: total}
	for _, u := range users {
		resp.Users = append(resp.Users, dto.ToAccountResp(u))
	}

	writeJSON(ctx, w, logger, http.StatusOK, resp)
}

// getUser returns a single account by id (admin).
//
//	@Summary		Get user (admin)
//	@Tags			identity-admin
//	@Produce		json
//	@Param			id	path		string	true	"Account UUID"
//	@Success		200	{object}	dto.AccountResp
//	@Failure		404	{object}	dto.ErrorResponse
//	@Router			/apis/admin/users/{id} [get]
func (h *accountHandler) getUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "GetUser")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	acc, err := h.account.Get(ctx, id)
	if err != nil {
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccountResp(acc))
}

// updateUser applies admin profile edits.
//
//	@Summary		Update user (admin)
//	@Description	Edit a user's handle and/or status. Tier changes go through the dedicated VIP/tier path.
//	@Tags			identity-admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Account UUID"
//	@Param			body	body		dto.UpdateUserReq	true	"Profile patch"
//	@Success		200		{object}	dto.AccountResp
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		404		{object}	dto.ErrorResponse
//	@Router			/apis/admin/users/{id} [patch]
func (h *accountHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "UpdateUser")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	var body dto.UpdateUserReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	patch := biz.UserPatch{Handle: body.Handle}
	if body.Status != nil {
		s := entity.Status(*body.Status)
		patch.Status = &s
	}

	acc, err := h.account.UpdateUser(ctx, id, patch)
	if err != nil {
		logger.ErrorContext(ctx, "update user failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccountResp(acc))
}

// assignRole grants a functional role (e.g. promote to INSPECTOR).
//
//	@Summary		Assign role (admin)
//	@Description	Grant a functional role. Promoting to INSPECTOR enables the inspector verification workflow. Emits account.role_changed.
//	@Tags			identity-admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Account UUID"
//	@Param			body	body		dto.AssignRoleReq	true	"Role to grant"
//	@Success		200		{object}	dto.AccountResp
//	@Failure		400		{object}	dto.ErrorResponse
//	@Failure		404		{object}	dto.ErrorResponse
//	@Router			/apis/admin/users/{id}/roles [post]
func (h *accountHandler) assignRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "AssignRole")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	var body dto.AssignRoleReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	acc, err := h.account.GrantRole(ctx, id, entity.Role(body.Role), actorID(r))
	if err != nil {
		logger.ErrorContext(ctx, "grant role failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccountResp(acc))
}

// revokeRole removes a functional role.
//
//	@Summary		Revoke role (admin)
//	@Tags			identity-admin
//	@Produce		json
//	@Param			id		path		string	true	"Account UUID"
//	@Param			role	path		string	true	"Role to revoke (INSPECTOR|ADMIN)"
//	@Success		200		{object}	dto.AccountResp
//	@Failure		400		{object}	dto.ErrorResponse
//	@Router			/apis/admin/users/{id}/roles/{role} [delete]
func (h *accountHandler) revokeRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := h.logger.With("method", "RevokeRole")

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, err), w)

		return
	}

	acc, err := h.account.RevokeRole(ctx, id, entity.Role(r.PathValue("role")), actorID(r))
	if err != nil {
		logger.ErrorContext(ctx, "revoke role failed", "error", err)
		dto.HandleError(err, w)

		return
	}

	writeJSON(ctx, w, logger, http.StatusOK, dto.ToAccountResp(acc))
}

// actorID extracts the acting admin's account id from the gateway-injected
// header (uuid.Nil if absent/unparseable — granted_by is then left null).
func actorID(r *http.Request) uuid.UUID {
	id, err := uuid.Parse(r.Header.Get(headerAccountID))
	if err != nil {
		return uuid.Nil
	}

	return id
}
