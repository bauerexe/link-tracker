package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability"
	"go.uber.org/zap"
)

type Relay struct {
	repo         *Repository
	producer     sarama.SyncProducer
	log          *zap.Logger
	batchSize    int
	pollInterval time.Duration
	maxAttempts  int
}

func NewOutboxRelay(
	pool *pgxpool.Pool,
	cfgKafka config.KafkaConfig,
	cfgSarama *sarama.Config,
	log *zap.Logger,
) (*Relay, error) {
	if log != nil {
		log = log.Named("outbox_relay")
	}

	cfgSarama.Producer.Return.Successes = true
	cfgSarama.Producer.Return.Errors = true

	producer, err := sarama.NewSyncProducer(cfgKafka.KafkaBrokers, cfgSarama)
	if err != nil {
		return nil, fmt.Errorf("new outbox sync producer: %w", err)
	}

	maxAttempts := cfgKafka.MaxRetries
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	const size = 100
	const i = 500
	return &Relay{
		repo:         NewRepository(pool),
		producer:     producer,
		log:          log,
		batchSize:    size,
		pollInterval: i * time.Millisecond,
		maxAttempts:  maxAttempts,
	}, nil
}

func (r *Relay) Run(ctx context.Context) error {
	r.log.Info("starting outbox relay")
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			r.processBatch(ctx)
		}
	}
}

func (r *Relay) Close() error {
	if r.producer == nil {
		return nil
	}

	return fmt.Errorf("error close: %w", r.producer.Close())
}

func (r *Relay) processBatch(ctx context.Context) {
	messages, err := r.repo.GetPending(ctx, r.batchSize)
	if err != nil {
		r.log.Error("get pending outbox messages failed", zap.Error(err))
		return
	}

	for _, msg := range messages {
		if err = r.publish(msg); err != nil {
			r.log.Error("publish outbox message failed",
				zap.Error(err),
				zap.Int64("outbox_id", msg.ID),
				zap.String("topic", msg.Topic),
				zap.Int("attempts", msg.Attempts),
			)

			if markErr := r.repo.MarkFailed(ctx, msg.ID, msg.Attempts, r.maxAttempts, err); markErr != nil {
				r.log.Error("mark outbox message failed",
					zap.Error(markErr),
					zap.Int64("outbox_id", msg.ID),
				)
			}

			continue
		}

		if err = r.repo.MarkPublished(ctx, msg.ID); err != nil {
			r.log.Error("mark outbox message published failed",
				zap.Error(err),
				zap.Int64("outbox_id", msg.ID),
			)
		}
	}
}

func (r *Relay) publish(msg Message) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeKafka, msg.Topic, started)

	_, _, err := r.producer.SendMessage(&sarama.ProducerMessage{
		Topic: msg.Topic,
		Key:   sarama.ByteEncoder(msg.MessageKey),
		Value: sarama.ByteEncoder(msg.Payload),
	})
	if err != nil {
		return fmt.Errorf("send kafka message from outbox: %w", err)
	}

	return nil
}
