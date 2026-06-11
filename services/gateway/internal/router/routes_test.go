package router_test

import (
	"testing"

	"application/internal/router"

	"github.com/stretchr/testify/require"
)

// TestMatch table-drives the gateway route table: public reads, access routes,
// admin prefixes, escrow-vs-dispute split, notifier WS, and (critically) the
// shared /apis/auctions/{id}/ prefix disambiguated by action suffix between the
// dutch (reserve|lock|buy) and passive (bid|standing) engines.
func TestMatch(t *testing.T) {
	t.Parallel()

	table := router.NewTable()

	tests := []struct {
		name         string
		method       string
		path         string
		wantUpstream string
		wantMatch    bool
		wantPublic   bool
		wantMember   bool
		wantKyc      bool
	}{
		// public
		{"weekly gallery", "GET", "/apis/gallery/weekly", "catalog", true, true, false, false},
		{"lot read", "GET", "/apis/lots/abc", "catalog", true, true, false, false},
		{"otp request", "POST", "/apis/auth/otp/request", "identity", true, true, false, false},
		{"otp verify", "POST", "/apis/auth/otp/verify", "identity", true, true, false, false},
		{"oauth callback", "GET", "/apis/auth/oauth/google/callback", "identity", true, true, false, false},
		{"kyc start", "POST", "/apis/kyc/start", "kyc", true, true, false, false},
		{"kyc verify", "POST", "/apis/kyc/verify", "kyc", true, true, false, false},

		// identity
		{"me", "GET", "/apis/me", "identity", true, false, false, false},
		{"internal access", "GET", "/apis/internal/accounts/x/access", "identity", true, false, false, false},
		{"admin user crud", "PATCH", "/apis/admin/users/x", "identity", true, false, false, false},

		// inspector workflow (catalog upstream)
		{"inspector queue", "GET", "/apis/inspector/queue", "catalog", true, false, false, false},
		{"inspector seal", "POST", "/apis/inspector/lots/x/inspect", "catalog", true, false, false, false},

		// kyc authed/admin
		{"kyc status", "GET", "/apis/kyc/status", "kyc", true, false, false, false},
		{"admin kyc queue", "GET", "/apis/admin/kyc", "kyc", true, false, false, false},
		{"admin kyc approve", "POST", "/apis/admin/kyc/x/approve", "kyc", true, false, false, false},

		// vault — participation gated
		{"vault list", "GET", "/apis/vault", "vault", true, false, true, true},
		{"vault object list", "POST", "/apis/vault/objects/x/list", "vault", true, false, true, true},

		// bids
		{"bids wallet", "GET", "/apis/bids/wallet", "bids", true, false, false, false},
		{"bids buy", "POST", "/apis/bids/buy", "bids", true, false, false, false},

		// auctions: dutch vs passive disambiguation
		{"dutch reserve", "POST", "/apis/auctions/A1/reserve", "auction-dutch", true, false, true, true},
		{"dutch lock", "POST", "/apis/auctions/A1/lock", "auction-dutch", true, false, true, true},
		{"dutch buy", "POST", "/apis/auctions/A1/buy", "auction-dutch", true, false, true, true},
		{"passive bid", "POST", "/apis/auctions/A1/bid", "auction-passive", true, false, true, true},
		{"passive standing", "GET", "/apis/auctions/A1/standing", "auction-passive", true, false, true, true},

		// admin auction actions
		{"admin open->dutch", "POST", "/apis/admin/auctions/A1/open", "auction-dutch", true, false, false, false},
		{"admin abort->dutch", "POST", "/apis/admin/auctions/A1/abort", "auction-dutch", true, false, false, false},
		{"admin close->passive", "POST", "/apis/admin/auctions/A1/close", "auction-passive", true, false, false, false},

		// escrow vs dispute
		{"escrow fund", "POST", "/apis/escrow/T1/fund", "escrow", true, false, true, true},
		{"escrow status", "GET", "/apis/escrow/T1", "escrow", true, false, true, true},
		{"dispute resolve->dispute", "POST", "/apis/escrow/T1/dispute/resolve", "dispute", true, false, false, false},
		{"admin escrow", "GET", "/apis/admin/escrow/list", "escrow", true, false, false, false},

		// notifier WS
		{"live ws", "GET", "/apis/live/auctions/A1", "notifier", true, false, false, false},

		// no match
		{"unknown path", "GET", "/apis/nope", "", false, false, false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			route, ok := table.Match(tc.method, tc.path)
			require.Equal(t, tc.wantMatch, ok, "match flag")

			if !tc.wantMatch {
				return
			}

			require.Equal(t, tc.wantUpstream, route.Upstream, "upstream")
			require.Equal(t, tc.wantPublic, route.Req.Public, "public")
			require.Equal(t, tc.wantMember, route.Req.RequireMember, "require member")
			require.Equal(t, tc.wantKyc, route.Req.RequireKyc, "require kyc")
		})
	}
}
