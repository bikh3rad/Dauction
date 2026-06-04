package router

import (
	"application/internal/biz"
	"sort"
	"strings"
)

// Route is one entry in the gateway's route table. A request is matched to a
// route by HTTP method + longest path prefix (see Table.Match). The matched
// route names the logical upstream service and the access requirement the guard
// enforces before proxying.
type Route struct {
	// Method is the HTTP method ("" / "*" matches any).
	Method string
	// Prefix is the path prefix under which the route applies (matched longest-first).
	Prefix string
	// Exact, when true, requires the request path to equal Prefix exactly.
	Exact bool
	// Suffix, when non-empty, requires the request path to end with this action
	// segment. Used to disambiguate the shared /apis/auctions/{id}/ prefix between
	// the dutch (reserve|lock|buy) and passive (bid|standing) engines.
	Suffix string
	// Upstream is the logical service name resolved via app.UpstreamsConfig.URL.
	Upstream string
	// Req is the access requirement the guard applies to this route.
	Req biz.RouteRequirement
}

// public is a participation-free requirement (no auth, no tier/KYC guard).
var public = biz.RouteRequirement{Public: true}

// participate requires MEMBER/VIP AND KYC APPROVED — the buyer/seller gate
// (root CLAUDE.md §1: "Participation needs MEMBER/VIP and KYC-approved").
var participate = biz.RouteRequirement{RequireMember: true, RequireKyc: true}

// authed requires a valid authenticated caller but no tier/KYC level (e.g.
// reading your own /me, wallet, escrow status). Not public, but not gated on tier.
var authed = biz.RouteRequirement{}

// Table is the ordered gateway route table. It is sorted by descending prefix
// length at construction so Match implements longest-prefix-wins; routes with a
// Suffix or Exact constraint are evaluated before plain-prefix routes of equal
// length so the dutch/passive disambiguation is deterministic.
type Table struct {
	routes []Route
}

// NewTable builds the canonical Dauction route table (root CLAUDE.md §6). Routes
// are sorted for longest-prefix-wins matching with suffix/exact specificity
// taking precedence at equal prefix length.
func NewTable() *Table {
	routes := []Route{
		// ---- public (no auth, no guard) ----
		{Method: "GET", Prefix: "/apis/gallery/", Upstream: "catalog", Req: public},
		{Method: "GET", Prefix: "/apis/gallery", Exact: true, Upstream: "catalog", Req: public},
		{Method: "GET", Prefix: "/apis/lots/", Upstream: "catalog", Req: public},
		{Method: "POST", Prefix: "/apis/invites/redeem", Upstream: "invite", Req: public},
		{Method: "POST", Prefix: "/apis/kyc/start", Upstream: "kyc", Req: public},
		{Method: "POST", Prefix: "/apis/kyc/verify", Upstream: "kyc", Req: public},

		// ---- identity (authed; reading own account / internal+admin) ----
		{Method: "GET", Prefix: "/apis/me", Exact: true, Upstream: "identity", Req: authed},
		{Prefix: "/apis/internal/accounts/", Upstream: "identity", Req: authed},
		{Prefix: "/apis/admin/accounts/", Upstream: "identity", Req: authed},

		// ---- invite (authed redemption beyond /redeem + admin) ----
		{Prefix: "/apis/admin/invites/", Upstream: "invite", Req: authed},
		{Prefix: "/apis/invites/", Upstream: "invite", Req: authed},

		// ---- kyc (authed status + admin queue) ----
		{Prefix: "/apis/admin/kyc/", Upstream: "kyc", Req: authed},
		{Prefix: "/apis/admin/kyc", Exact: true, Upstream: "kyc", Req: authed},
		{Prefix: "/apis/kyc/", Upstream: "kyc", Req: authed},

		// ---- vault (seller; participation-gated) ----
		{Prefix: "/apis/vault/", Upstream: "vault", Req: participate},
		{Prefix: "/apis/vault", Exact: true, Upstream: "vault", Req: participate},

		// ---- bids (wallet read + purchase; authed) ----
		{Prefix: "/apis/bids/", Upstream: "bids", Req: authed},

		// ---- auctions: shared /apis/auctions/{id}/ prefix, disambiguated by suffix ----
		// dutch (live): reserve | lock | buy
		{Prefix: "/apis/auctions/", Suffix: "/reserve", Upstream: "auction-dutch", Req: participate},
		{Prefix: "/apis/auctions/", Suffix: "/lock", Upstream: "auction-dutch", Req: participate},
		{Prefix: "/apis/auctions/", Suffix: "/buy", Upstream: "auction-dutch", Req: participate},
		// passive (timed): bid | standing
		{Prefix: "/apis/auctions/", Suffix: "/bid", Upstream: "auction-passive", Req: participate},
		{Prefix: "/apis/auctions/", Suffix: "/standing", Upstream: "auction-passive", Req: participate},
		// admin auction actions: open|complete|abort → dutch; close → passive
		{Prefix: "/apis/admin/auctions/", Suffix: "/open", Upstream: "auction-dutch", Req: authed},
		{Prefix: "/apis/admin/auctions/", Suffix: "/complete", Upstream: "auction-dutch", Req: authed},
		{Prefix: "/apis/admin/auctions/", Suffix: "/abort", Upstream: "auction-dutch", Req: authed},
		{Prefix: "/apis/admin/auctions/", Suffix: "/close", Upstream: "auction-passive", Req: authed},

		// ---- escrow + dispute ----
		// dispute court routes are mounted under /apis/escrow/{id}/dispute → dispute service
		{Prefix: "/apis/escrow/", Suffix: "/dispute/resolve", Upstream: "dispute", Req: authed},
		{Prefix: "/apis/admin/escrow/", Upstream: "escrow", Req: authed},
		{Prefix: "/apis/escrow/", Upstream: "escrow", Req: participate},

		// ---- notifier (realtime WS/SSE) ----
		{Prefix: "/apis/live/", Upstream: "notifier", Req: authed},
	}

	t := &Table{routes: routes}
	t.sortRoutes()

	return t
}

// sortRoutes orders the table for deterministic matching: a Suffix/Exact
// constraint is more specific (evaluated first); within equal specificity,
// longer prefixes win.
func (t *Table) sortRoutes() {
	sort.SliceStable(t.routes, func(i, j int) bool {
		ri, rj := t.routes[i], t.routes[j]

		si, sj := specificity(ri), specificity(rj)
		if si != sj {
			return si > sj
		}

		if len(ri.Prefix) != len(rj.Prefix) {
			return len(ri.Prefix) > len(rj.Prefix)
		}

		return len(ri.Suffix) > len(rj.Suffix)
	})
}

// specificity ranks routes: suffix-constrained (most specific) > exact > plain prefix.
func specificity(r Route) int {
	switch {
	case r.Suffix != "":
		return 2
	case r.Exact:
		return 1
	default:
		return 0
	}
}

// Match returns the first route (in specificity/longest-prefix order) whose
// method and path constraints satisfy the request, plus whether a match was found.
func (t *Table) Match(method, path string) (Route, bool) {
	for _, r := range t.routes {
		if !methodMatches(r.Method, method) {
			continue
		}

		if r.Exact {
			if path == r.Prefix {
				return r, true
			}

			continue
		}

		if !strings.HasPrefix(path, r.Prefix) {
			continue
		}

		if r.Suffix != "" && !strings.HasSuffix(path, r.Suffix) {
			continue
		}

		return r, true
	}

	return Route{}, false
}

// Routes exposes the ordered table (read-only) for docs/tests.
func (t *Table) Routes() []Route {
	out := make([]Route, len(t.routes))
	copy(out, t.routes)

	return out
}

func methodMatches(routeMethod, reqMethod string) bool {
	if routeMethod == "" || routeMethod == "*" {
		return true
	}

	return strings.EqualFold(routeMethod, reqMethod)
}
