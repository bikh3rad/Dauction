package datasource

import (
	"application/app"
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var errNatsDisconnected = errors.New("nats disconnected")

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

func NewNats(ctx context.Context, logger *slog.Logger, config *app.KConfig, controller app.Controller) (*Nats, error) {
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

	natsDS := &Nats{
		logger:    logger.With("module", "datasource.nats"),
		Client:    nc,
		JetStream: js,
	}

	if cfg.InitJetstream {
		if err := natsDS.initJetStream(ctx, cfg); err != nil {
			logger.Error("Failed to initialize JetStream", "error", err)

			return nil, err
		}
	} else if cfg.StreamName != "" {
		logger.Info("JetStream initialization skipped")

		stream, err := js.Stream(ctx, cfg.StreamName)
		if err != nil {
			logger.Error("Failed to get existing NATS stream", "stream_name", cfg.StreamName, "error", err)

			return nil, err
		}

		natsDS.Stream = stream
	}

	controller.RegisterHealthz("nats", func(_ context.Context) error {
		if nc.IsConnected() {
			return nil
		}

		return errNatsDisconnected
	})
	controller.RegisterShutdown("nats", func(_ context.Context) error {
		nc.Close()

		return nil
	})

	return natsDS, nil
}

func (n *Nats) initJetStream(ctx context.Context, cfg *NatsConfig) error {
	streamConfig := jetstream.StreamConfig{
		Name:        cfg.StreamName,
		Subjects:    strings.Split(cfg.Subjects, ","),
		Description: "Dauction domain event stream",
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
