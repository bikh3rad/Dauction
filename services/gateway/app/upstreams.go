package app

import (
	"context"
	"fmt"
)

// UpstreamsConfig maps each backend service name to its base URL. Every value is
// overridable via APP_UPSTREAMS_<NAME> (koanf `APP_`-prefixed env). The router
// resolves a matched route to one of these names.
type UpstreamsConfig struct {
	Identity       string `koanf:"identity"`
	Invite         string `koanf:"invite"`
	Kyc            string `koanf:"kyc"`
	Vault          string `koanf:"vault"`
	Catalog        string `koanf:"catalog"`
	AuctionDutch   string `koanf:"auction-dutch"`
	AuctionPassive string `koanf:"auction-passive"`
	Bids           string `koanf:"bids"`
	Escrow         string `koanf:"escrow"`
	Dispute        string `koanf:"dispute"`
	Notifier       string `koanf:"notifier"`
}

// NewUpstreamsConfig loads the upstream service URL map from the `upstreams`
// config sub-tree, defaulting to the docker-compose container DNS names.
func NewUpstreamsConfig(_ context.Context, c *KConfig) (*UpstreamsConfig, error) {
	cfg := &UpstreamsConfig{
		Identity:       "http://identity:8080",
		Invite:         "http://invite:8080",
		Kyc:            "http://kyc:8080",
		Vault:          "http://vault:8080",
		Catalog:        "http://catalog:8080",
		AuctionDutch:   "http://auction-dutch:8080",
		AuctionPassive: "http://auction-passive:8080",
		Bids:           "http://bids:8080",
		Escrow:         "http://escrow:8080",
		Dispute:        "http://dispute:8080",
		Notifier:       "http://notifier:8080",
	}
	if err := c.Unmarshal("upstreams", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// URL returns the base URL for a logical upstream name, or an error if unknown.
func (u *UpstreamsConfig) URL(name string) (string, error) {
	switch name {
	case "identity":
		return u.Identity, nil
	case "invite":
		return u.Invite, nil
	case "kyc":
		return u.Kyc, nil
	case "vault":
		return u.Vault, nil
	case "catalog":
		return u.Catalog, nil
	case "auction-dutch":
		return u.AuctionDutch, nil
	case "auction-passive":
		return u.AuctionPassive, nil
	case "bids":
		return u.Bids, nil
	case "escrow":
		return u.Escrow, nil
	case "dispute":
		return u.Dispute, nil
	case "notifier":
		return u.Notifier, nil
	default:
		return "", fmt.Errorf("unknown upstream %q", name)
	}
}
