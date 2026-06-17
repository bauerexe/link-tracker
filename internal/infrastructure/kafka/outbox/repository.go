package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/observability"
)

const (
	insertOutboxTx = `
		INSERT INTO outbox_messages (
			topic,
			message_key,
			payload
		)
		VALUES ($1, $2, $3)
	`
	selectOutboxTx = `
		SELECT 
			id,
			topic,
			message_key,
			payload,
			attempts
		FROM outbox_messages
		WHERE status = 'pending'
		ORDER BY id
		LIMIT $1
	`
	updateMarkPublishedTx = `
		UPDATE outbox_messages
		SET 
			status = 'published',
			published_at = NOW(),
			last_error = NULL
		WHERE id = $1
	`
	updateMarkFailedTx = `
			UPDATE outbox_messages
			SET 
				status = 'failed',
				attempts = attempts + 1,
				last_error = $2
			WHERE id = $1
		`
	updateMarkPendingTx = `
		UPDATE outbox_messages
		SET 
			status = 'pending',
			attempts = attempts + 1,
			last_error = $2
		WHERE id = $1
	`
)

type Message struct {
	ID         int64
	Topic      string
	MessageKey []byte
	Payload    []byte
	Attempts   int
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func InsertTx(ctx context.Context, tx pgx.Tx, msg Message) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "outbox_messages", started)

	_, err := tx.Exec(ctx, insertOutboxTx,
		msg.Topic,
		msg.MessageKey,
		msg.Payload,
	)
	if err != nil {
		return fmt.Errorf("insert outbox message: %w", err)
	}

	return nil
}

func (r *Repository) GetPending(ctx context.Context, limit int) ([]Message, error) {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "outbox_messages", started)

	rows, err := r.pool.Query(ctx, selectOutboxTx, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending outbox messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0)

	for rows.Next() {
		var msg Message

		err = rows.Scan(
			&msg.ID,
			&msg.Topic,
			&msg.MessageKey,
			&msg.Payload,
			&msg.Attempts,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox message: %w", err)
		}

		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox messages: %w", err)
	}

	return messages, nil
}

func (r *Repository) MarkPublished(ctx context.Context, id int64) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "outbox_messages", started)

	_, err := r.pool.Exec(ctx, updateMarkPublishedTx, id)
	if err != nil {
		return fmt.Errorf("mark outbox message published: %w", err)
	}

	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, id int64, attempts int, maxAttempts int, cause error) error {
	started := time.Now()
	defer observability.ObserveRequestDuration(observability.ScopeDatabase, "outbox_messages", started)

	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}

	if attempts+1 >= maxAttempts {
		_, err := r.pool.Exec(ctx, updateMarkFailedTx, id, lastError)
		if err != nil {
			return fmt.Errorf("mark outbox message failed: %w", err)
		}

		return nil
	}

	_, err := r.pool.Exec(ctx, updateMarkPendingTx, id, lastError)
	if err != nil {
		return fmt.Errorf("increase outbox message attempts: %w", err)
	}

	return nil
}
