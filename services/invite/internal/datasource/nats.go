package datasource

import (
	"application/app"
	"context"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Nats struct {
	logger    *slog.Logger
	Client    *nats.Conn
	JetStream jetstream.JetStream
	Stream    jetstream.Stream
	// Protocol  *cejsm.Protocol
}

type NatsConfig struct {
	DSN           string `json:"dsn"`
	InitJetstream bool   `json:"initJetstream"`
	StreamName    string `json:"streamName"`
	Subjects      string `json:"subject"`
}

func NewNats(ctx context.Context, logger *slog.Logger, config *app.KConfig) (*Nats, error) {
	cfg := new(NatsConfig)
	if err := config.Unmarshal("datasource.nats", cfg); err != nil {
		logger.Error("Failed to unmarshal NATS config", "error", err)

		return nil, err
	}

	nc, err := nats.Connect(cfg.DSN)
	if err != nil {
		logger.Error("Failed to connect to NATS", "error", err)

		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		logger.Error("Failed to create JetStream context", "error", err)

		return nil, err
	}

	nats := &Nats{
		logger:    logger.With("module", "datasource.nats"),
		Client:    nc,
		JetStream: js,
	}

	if cfg.InitJetstream {
		if err := nats.initJetStream(ctx, cfg); err != nil {
			logger.Error("Failed to initialize JetStream", "error", err)

			return nil, err
		}
	} else {
		logger.Info("JetStream initialization skipped")

		stream, err := js.Stream(ctx, cfg.StreamName)
		if err != nil {
			logger.Error("Failed to get existing NATS stream", "stream_name", cfg.StreamName, "error", err)

			return nil, err
		}

		nats.Stream = stream
	}

	return nats, nil
}

func (n *Nats) initJetStream(ctx context.Context, cfg *NatsConfig) error {
	// The DAUCTION stream is SHARED by every service. All services create-or-update
	// it with an IDENTICAL config (file storage, limits retention, the full
	// platform subject list) so startup order never matters and every domain
	// subject is always bound. Publishers target specific subjects; consumers bind
	// durable filters on their own subset.
	streamConfig := jetstream.StreamConfig{
		Name:        cfg.StreamName,
		Subjects:    splitSubjects(cfg.Subjects),
		Description: "Dauction domain events",
		Storage:     jetstream.FileStorage,
		Replicas:    1,
		Retention:   jetstream.LimitsPolicy,
	}

	s, err := n.JetStream.CreateOrUpdateStream(ctx, streamConfig)
	if err != nil {
		n.logger.Error("Failed to add NATS stream", "error", err)

		return err
	}

	n.Stream = s

	n.logger.Info("NATS stream initialized successfully", "stream_name", cfg.StreamName)

	return nil
}

// splitSubjects parses the comma-separated subjects config into a trimmed,
// non-empty list (an empty entry is an invalid NATS subject).
func splitSubjects(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}

	return out
}
