package handler_test

import (
	"application/internal/biz"
	"application/internal/entity"
	"application/internal/service/handler"
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newLiveServer wires the real Hub + Projector behind the SSE handler on an
// httptest server, returning the server, the projector (to inject events), and the
// clock instant used for determinism.
func newLiveServer(t *testing.T) (*httptest.Server, *biz.Projector) {
	t.Helper()

	hub := biz.NewHub(discardLogger())
	reg := biz.NewRegistry()
	clock := fixedClock{t: time.Date(2026, 6, 1, 12, 0, 30, 0, time.UTC)}
	proj := biz.NewProjector(discardLogger(), hub, reg, clock)
	sub := biz.NewSubscriber(hub, proj)

	mux := http.NewServeMux()
	h := handler.NewLiveHandler(discardLogger(), mux, sub)
	require.NoError(t, h.RegisterHandler(context.Background()))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, proj
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// readFrame reads one SSE event block (lines until a blank line) from the stream.
func readFrame(t *testing.T, br *bufio.Reader) string {
	t.Helper()

	var b strings.Builder

	for {
		line, err := br.ReadString('\n')
		require.NoError(t, err)

		if line == "\n" { // end of frame
			if b.Len() == 0 {
				continue // skip leading/keep-alive blanks
			}

			return b.String()
		}

		if strings.HasPrefix(line, ":") { // keep-alive comment
			continue
		}

		b.WriteString(line)
	}
}

// TestLiveAuctionSnapshotOnConnect asserts that connecting to an open Dutch
// auction's room immediately yields a SNAPSHOT frame carrying the computed price.
func TestLiveAuctionSnapshotOnConnect(t *testing.T) {
	t.Parallel()

	srv, proj := newLiveServer(t)

	// Track an open Dutch auction (opened 30s before the fixed clock) so the
	// connect-time snapshot carries a computed, mid-descent price.
	openAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, proj.Handle(context.Background(), openedEnvelope(t, "a1", openAt)))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/apis/live/auctions/a1", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	frame := readFrame(t, bufio.NewReader(resp.Body))
	require.Contains(t, frame, "event: "+entity.KindSnapshot)
	require.Contains(t, frame, "\"kind\":\""+entity.KindSnapshot+"\"")
	require.Contains(t, frame, "\"currentPriceCents\":97000")
}

// TestLiveMeRequiresAccountHeader asserts the me-feed rejects a missing
// X-Account-Id with 401.
func TestLiveMeRequiresAccountHeader(t *testing.T) {
	t.Parallel()

	srv, _ := newLiveServer(t)

	resp, err := http.Get(srv.URL + "/apis/live/me")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestLiveAuctionBroadcastReachesClient asserts an event injected after connect is
// streamed to the connected client.
func TestLiveAuctionBroadcastReachesClient(t *testing.T) {
	t.Parallel()

	srv, proj := newLiveServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/apis/live/auctions/a9", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	br := bufio.NewReader(resp.Body)

	// Give the handler a moment to register the client, then inject a hammer.
	require.Eventually(t, func() bool {
		return proj != nil
	}, time.Second, 10*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	require.NoError(t, proj.Handle(context.Background(), hammerEnvelope(t, "a9")))

	frame := readFrame(t, br)
	require.Contains(t, frame, "event: "+entity.KindHammer)
	require.Contains(t, frame, "\"winnerId\":\"w1\"")
}
