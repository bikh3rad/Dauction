package biz

import (
	"application/app"

	"github.com/google/wire"
)

// NewInviteConfig reads the invite-service policy from the `invite` config sub-tree.
func NewInviteConfig(c *app.KConfig) (InviteConfig, error) {
	cfg := InviteConfig{IssueQuota: 5} //nolint:mnd // default house policy
	if c.Exists("invite") {
		if err := c.Unmarshal("invite", &cfg); err != nil {
			return InviteConfig{}, err
		}
	}

	return cfg, nil
}

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewInviteConfig,
	NewInvite,
	wire.Bind(new(UsecaseInvite), new(*invite)),
)
