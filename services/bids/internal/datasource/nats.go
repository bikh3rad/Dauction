package datasource

import (
	"application/app"
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Nats is the JetStream client. bids only PUBLISHES (it is the EventPublisher the
// outbox relay targets); it consumes nothing — auction-passive calls it
// synchronously (CLAUDE.md §2, §5).
type Nats struct {
	logger   *slog.Logger
	conn     *nats.Conn
	js       jetstream.JetStream
	stream   jetstream.Stream
	subjects []string
}

// NatsConfig is the datasource.nats sub-tree (koanf). DSN is the NATS URL;
// StreamName is the shared Dauction event stream; Subjects is the comma list of
// subjects this stream binds (the platform stream is created idempotently).
type NatsConfig struct {
	DSN        string `koanf:"dsn"`
	StreamName string `koanf:"streamName"`
	Subjects   string `koanf:"subjects"`
	Durable    string `koanf:"durable"`
}

// NewNats connects to NATS, ensures the shared event stream exists, and registers
// healthz + shutdown on the controller.
func NewNats(ctx context.Context, logger *slog.Logger, config *app.KConfig, controller app.Controller) (*Nats, error) {
	cfg := new(NatsConfig)
	if err := config.Unmarshal("datasource.nats", cfg); err != nil {
		logger.Error("failed to unmarshal NATS config", "error", err)

		return nil, err
	}

	nc, err := nats.Connect(cfg.DSN,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second), //nolint:mnd
	)
	if err != nil {
		logger.Error("failed to connect to NATS", "error", err, "dsn", cfg.DSN)

		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Error("failed to create JetStream context", "error", err)

		return nil, err
	}

	subjects := splitCSV(cfg.Subjects)

	n := &Nats{
		logger:   logger.With("layer", "Nats"),
		conn:     nc,
		js:       js,
		subjects: subjects,
	}

	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        cfg.StreamName,
		Subjects:    subjects,
		Description: "Dauction domain events",
		Storage:     jetstream.FileStorage,
		Replicas:    1,
		Retention:   jetstream.LimitsPolicy,
	})
	if err != nil {
		logger.Error("failed to ensure NATS stream", "error", err, "stream", cfg.StreamName)

		return nil, err
	}

	n.stream = stream

	controller.RegisterHealthz("nats", n.healthz)
	controller.RegisterShutdown("nats", n.shutdown)

	return n, nil
}

// Publish implements biz.EventPublisher: publishes payload to subject on
// JetStream. Used by the outbox relay.
func (n *Nats) Publish(ctx context.Context, subject string, payload []byte) error {
	_, err := n.js.Publish(ctx, subject, payload)

	return err
}

func (n *Nats) healthz(_ context.Context) error {
	if n.conn == nil || !n.conn.IsConnected() {
		return nats.ErrConnectionClosed
	}

	return nil
}

func (n *Nats) shutdown(_ context.Context) error {
	n.logger.Info("shutting down NATS")
	n.conn.Close()

	return nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}
