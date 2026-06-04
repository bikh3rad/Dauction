package handler

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/service"
	"application/internal/service/dto"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// keepAliveInterval bounds idle SSE connections: a comment ping every interval
// keeps proxies from reaping the stream and lets the server detect a dead client.
const keepAliveInterval = 25 * time.Second

// liveHandler serves the realtime feed over Server-Sent Events (SSE). SSE is the
// chosen transport (not WebSocket): the feed is strictly server→client (the socket
// is a read-only view per root CLAUDE.md §6), and SSE needs no extra dependency —
// the Go toolchain is blocked in this environment, so adding a WS library to
// go.mod is not viable. SSE over plain net/http is the safe, dependency-free
// default and still hits the <100ms broadcast target.
type liveHandler struct {
	logger *slog.Logger
	mux    *http.ServeMux
	live   biz.UsecaseLive
}

var _ service.Handler = (*liveHandler)(nil)

// NewLiveHandler creates the SSE live handler.
func NewLiveHandler(logger *slog.Logger, mux *http.ServeMux, live biz.UsecaseLive) *liveHandler {
	return &liveHandler{
		logger: logger.With("layer", "LiveHandler"),
		mux:    mux,
		live:   live,
	}
}

// RegisterHandler registers the SSE routes (Go 1.22 method patterns).
func (h *liveHandler) RegisterHandler(_ context.Context) error {
	h.mux.HandleFunc("GET /apis/live/auctions/{id}", h.auction)
	h.mux.HandleFunc("GET /apis/live/me", h.me)

	return nil
}

// auction streams an auction room's live view-state.
//
//	@Summary		Live auction feed (SSE)
//	@Description	Server-Sent Events stream of server-computed view-state for an auction: live Dutch price + next drop, passive countdown/standing, hammer/resolution, and escrow state. A snapshot frame is sent on connect. Read-only — the socket never accepts input.
//	@Tags			live
//	@Produce		text/event-stream
//	@Param			id	path	string	true	"Auction ID"
//	@Success		200	"event stream"
//	@Router			/apis/live/auctions/{id} [get]
func (h *liveHandler) auction(w http.ResponseWriter, r *http.Request) {
	auctionID := r.PathValue("id")
	if auctionID == "" {
		dto.HandleError(errors.Join(biz.ErrResourceInvalid, errors.New("missing auction id")), w)

		return
	}

	room := biz.AuctionRoomKey(auctionID)
	snapshot := h.live.AuctionSnapshot(auctionID)

	h.stream(w, r, room, snapshot)
}

// me streams the caller's personal feed (escrow state, personal toasts), keyed by
// the gateway-injected X-Account-Id header.
//
//	@Summary		Personal live feed (SSE)
//	@Description	Server-Sent Events stream of the caller's personal view-state (escrow trade state changes, personal toasts). Caller identity comes from the gateway-injected X-Account-Id header. Read-only.
//	@Tags			live
//	@Produce		text/event-stream
//	@Param			X-Account-Id	header	string	true	"Caller account id (gateway-injected)"
//	@Success		200	"event stream"
//	@Failure		401	{object}	dto.ErrorResponse	"missing X-Account-Id"
//	@Router			/apis/live/me [get]
func (h *liveHandler) me(w http.ResponseWriter, r *http.Request) {
	accountID := r.Header.Get("X-Account-Id")
	if accountID == "" {
		dto.HandleError(errors.Join(biz.ErrResourceAccessDenied, errors.New("missing X-Account-Id")), w)

		return
	}

	h.stream(w, r, biz.MeRoomKey(accountID), nil)
}

// stream upgrades the response to an SSE event stream: it sets the headers, sends
// an optional snapshot frame, then fans every room message to the wire until the
// client disconnects. A keep-alive comment is written periodically.
func (h *liveHandler) stream(w http.ResponseWriter, r *http.Request, room string, snapshot *entity.Message) {
	ctx := r.Context()
	logger := h.logger.With("room", room)

	flusher, ok := w.(http.Flusher)
	if !ok {
		dto.HandleError(errors.New("streaming unsupported"), w)

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	client := h.live.Subscribe(room)
	defer h.live.Unsubscribe(client)

	logger.DebugContext(ctx, "sse client connected")

	if snapshot != nil {
		if err := writeEvent(w, *snapshot); err != nil {
			logger.WarnContext(ctx, "failed to write snapshot", "error", err)

			return
		}

		flusher.Flush()
	}

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.DebugContext(ctx, "sse client disconnected")

			return
		case msg, open := <-client.C:
			if !open {
				return
			}

			if err := writeEvent(w, msg); err != nil {
				logger.WarnContext(ctx, "failed to write event", "error", err)

				return
			}

			flusher.Flush()
		case <-keepAlive.C:
			if _, err := w.Write([]byte(": keep-alive\n\n")); err != nil {
				return
			}

			flusher.Flush()
		}
	}
}

// writeEvent serialises a Message as a single SSE `data:` frame.
func writeEvent(w http.ResponseWriter, msg entity.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if _, err := w.Write([]byte("event: " + msg.Kind + "\n")); err != nil {
		return err
	}

	if _, err := w.Write([]byte("data: ")); err != nil {
		return err
	}

	if _, err := w.Write(body); err != nil {
		return err
	}

	_, err = w.Write([]byte("\n\n"))

	return err
}
