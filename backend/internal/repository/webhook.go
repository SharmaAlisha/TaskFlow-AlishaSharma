package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/taskflow/backend/internal/model"
)

type WebhookRepository struct {
	pool *pgxpool.Pool
}

func NewWebhookRepository(pool *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{pool: pool}
}

func (r *WebhookRepository) Create(ctx context.Context, sub *model.WebhookSubscription) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO webhook_subscriptions (id, user_id, url, secret, event_types, project_ids, active, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sub.ID, sub.UserID, sub.URL, sub.Secret, sub.EventTypes, sub.ProjectIDs, sub.Active, sub.CreatedAt,
	)
	return err
}

func (r *WebhookRepository) FindByUser(ctx context.Context, userID string) ([]model.WebhookSubscription, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, url, secret, event_types, project_ids, active, created_at
		 FROM webhook_subscriptions WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []model.WebhookSubscription
	for rows.Next() {
		var s model.WebhookSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.Secret, &s.EventTypes, &s.ProjectIDs, &s.Active, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *WebhookRepository) FindByID(ctx context.Context, id string) (*model.WebhookSubscription, error) {
	var s model.WebhookSubscription
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, url, secret, event_types, project_ids, active, created_at
		 FROM webhook_subscriptions WHERE id = $1`, id,
	).Scan(&s.ID, &s.UserID, &s.URL, &s.Secret, &s.EventTypes, &s.ProjectIDs, &s.Active, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *WebhookRepository) Delete(ctx context.Context, id, userID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM webhook_subscriptions WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *WebhookRepository) FindMatchingSubscriptions(ctx context.Context, eventType string, projectID string) ([]model.WebhookSubscription, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, url, secret, event_types, project_ids, active, created_at
		 FROM webhook_subscriptions
		 WHERE active = true
		   AND $1 = ANY(event_types)
		   AND $2 = ANY(project_ids)`,
		eventType, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []model.WebhookSubscription
	for rows.Next() {
		var s model.WebhookSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.URL, &s.Secret, &s.EventTypes, &s.ProjectIDs, &s.Active, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
