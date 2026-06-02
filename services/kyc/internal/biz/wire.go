package biz

import (
	"application/app"
	"context"
	"time"

	"github.com/google/wire"
)

// NewKycConfig builds the OTP/event Config from koanf.
func NewKycConfig(kcfg *app.KConfig) (Config, error) {
	var raw struct {
		OTPTTLSeconds int  `koanf:"otpTtlSeconds"`
		MaxAttempts   int  `koanf:"maxAttempts"`
		Production    bool `koanf:"production"`
	}

	if err := kcfg.Unmarshal("kyc", &raw); err != nil {
		return Config{}, err
	}

	cfg := Config{
		MaxAttempts: raw.MaxAttempts,
		Production:  raw.Production,
	}

	if raw.OTPTTLSeconds > 0 {
		cfg.OTPTTL = time.Duration(raw.OTPTTLSeconds) * time.Second
	}

	return cfg, nil
}

// RegisterOutboxRelay starts the outbox relay as a background component. It is a
// provider whose only purpose is the side effect of registering the relay loop
// on the app Controller; it returns a marker so Wire keeps it in the graph.
func RegisterOutboxRelay(controller app.Controller, relay *OutboxRelay) OutboxRelayMarker {
	controller.RegisterStartup("kyc-outbox-relay", func(ctx context.Context) error {
		go relay.Run(ctx)

		return nil
	})

	return OutboxRelayMarker{}
}

// OutboxRelayMarker is a zero-value marker so Wire includes RegisterOutboxRelay.
type OutboxRelayMarker struct{}

var BizProviderSet = wire.NewSet(
	NewHealthz,
	wire.Bind(new(UsecaseHealthzer), new(*healthz)),

	NewKycConfig,
	NewKyc,
	wire.Bind(new(UsecaseKyc), new(*kyc)),

	NewOutboxRelay,
	RegisterOutboxRelay,
)
