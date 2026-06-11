#!/usr/bin/env bash
# Dauction end-to-end smoke test. Brings nothing up itself — run after `make up`
# once the stack is healthy. Exercises the public gateway + a few service surfaces.
# Usage: deploy/smoke.sh   (GW defaults to localhost:18080)
set -u

GW="${GW:-http://localhost:18080}"
pass=0 fail=0

check() { # name expected_substr actual
  if printf '%s' "$3" | grep -q "$2"; then
    printf '  ✓ %s\n' "$1"; pass=$((pass+1))
  else
    printf '  ✗ %s\n     want ~ %q\n     got    %q\n' "$1" "$2" "$3"; fail=$((fail+1))
  fi
}

echo "== gateway health =="
check "gateway liveness" "ok" "$(curl -s -m5 "$GW/healthz/liveness")"

echo "== public routes (no auth) — proxied to catalog =="
# Weekly gallery is public; empty DB returns an empty list, not an auth error.
g=$(curl -s -m5 -o /dev/null -w '%{http_code}' "$GW/apis/gallery/weekly")
check "GET /apis/gallery/weekly reachable (2xx)" "20" "$g"

echo "== guard: participation route without auth is rejected =="
u=$(curl -s -m5 -o /dev/null -w '%{http_code}' "$GW/apis/vault")
check "GET /apis/vault unauthenticated -> 401" "401" "$u"

echo "== dev auth: bearer = account id, gateway injects X-Account-Id =="
ACC="11111111-1111-1111-1111-111111111111"
me=$(curl -s -m5 -H "Authorization: Bearer $ACC" "$GW/apis/me")
# identity auto-provisions/serves the caller's read model; expect the id echoed or a JSON body.
check "GET /apis/me with bearer returns JSON" "{" "$me"

echo "== bids packages (public catalogue, seeded) =="
pk=$(curl -s -m5 "$GW/apis/bids/packages")
check "bids packages seeded (PKG_100)" "PKG_100" "$pk"

echo
echo "== per-service direct health (debug ports) =="
declare -A ports=(
  [identity]=18081 [kyc]=18083 [vault]=18084 [catalog]=18085
  [bids]=18086 [auction-dutch]=18087 [auction-passive]=18088 [escrow]=18089
  [dispute]=18090 [notifier]=18091
)
for svc in "${!ports[@]}"; do
  h=$(curl -s -m5 "http://localhost:${ports[$svc]}/healthz/liveness")
  check "$svc liveness" "ok" "$h"
done

echo
echo "result: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
