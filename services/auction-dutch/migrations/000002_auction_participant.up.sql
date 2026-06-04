-- auction_participant: an account's standing in a single auction. Caches the
-- gateway-supplied eligibility (KYC + tier) and the two lock states gating a buy
-- (root CLAUDE.md §3). reservation_state mirrors the 10% deposit lock;
-- full_lock_state mirrors the 100% full lock. Both must be LOCKED (alongside
-- KYC ∧ tier ∈ {MEMBER,VIP}) to buy.
CREATE TABLE auction_participant (
    auction_id        UUID        NOT NULL REFERENCES auction (id) ON DELETE CASCADE,
    account_id        UUID        NOT NULL,
    kyc_approved      BOOLEAN     NOT NULL DEFAULT FALSE,
    tier              TEXT        NOT NULL DEFAULT 'GUEST'
                          CHECK (tier IN ('GUEST', 'MEMBER', 'VIP')),
    reservation_state TEXT        NOT NULL DEFAULT 'REQUESTED'
                          CHECK (reservation_state IN ('REQUESTED', 'LOCKED', 'RELEASED')),
    full_lock_state   TEXT        NOT NULL DEFAULT 'REQUESTED'
                          CHECK (full_lock_state IN ('REQUESTED', 'LOCKED', 'RELEASED')),
    joined_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (auction_id, account_id)
);

-- Entry-to-OPEN / buy gate scan: count fully-eligible participants per auction.
CREATE INDEX idx_participant_eligible
    ON auction_participant (auction_id, reservation_state, full_lock_state);
