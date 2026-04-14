package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/taskflow/backend/internal/model"
)

type TaskFilter struct {
	Status   string
	Assignee string
}

type AssigneeCount struct {
	AssigneeID *string
	Count      int
}

type TaskRepository struct {
	pool *pgxpool.Pool
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

func (r *TaskRepository) Create(ctx context.Context, t *model.Task) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tasks (id, title, description, status, priority, project_id, creator_id, assignee_id, due_date, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, $10, $11)`,
		t.ID, t.Title, t.Description, t.Status, t.Priority,
		t.ProjectID, t.CreatorID, t.AssigneeID, t.DueDate,
		t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *TaskRepository) FindByID(ctx context.Context, id string) (*model.Task, error) {
	var t model.Task
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, description, status, priority, project_id, creator_id, assignee_id, due_date::text, version, created_at, updated_at
		 FROM tasks WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.ProjectID, &t.CreatorID, &t.AssigneeID, &t.DueDate,
		&t.Version, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskRepository) FindByProject(ctx context.Context, projectID string, filter TaskFilter, limit, offset int) ([]model.Task, int, error) {
	where := []string{"project_id = $1"}
	args := []interface{}{projectID}
	argIdx := 2

	if filter.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Assignee != "" {
		where = append(where, fmt.Sprintf("assignee_id = $%d", argIdx))
		args = append(args, filter.Assignee)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tasks WHERE %s", whereClause)
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, title, description, status, priority, project_id, creator_id, assignee_id, due_date::text, version, created_at, updated_at
		 FROM tasks WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.ProjectID, &t.CreatorID, &t.AssigneeID, &t.DueDate,
			&t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (r *TaskRepository) Update(ctx context.Context, id string, version int, updates map[string]interface{}) (*model.Task, error) {
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	for col, val := range updates {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	setClauses = append(setClauses, "version = version + 1", "updated_at = NOW()")

	query := fmt.Sprintf(
		`UPDATE tasks SET %s WHERE id = $%d AND version = $%d
		 RETURNING id, title, description, status, priority, project_id, creator_id, assignee_id, due_date::text, version, created_at, updated_at`,
		strings.Join(setClauses, ", "), argIdx, argIdx+1,
	)
	args = append(args, id, version)

	var t model.Task
	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.ProjectID, &t.CreatorID, &t.AssigneeID, &t.DueDate,
			&t.Version, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TaskRepository) Delete(ctx context.Context, id string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *TaskRepository) CountByStatus(ctx context.Context, projectID string) (map[string]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT status, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]int{"todo": 0, "in_progress": 0, "done": 0}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, rows.Err()
}

func (r *TaskRepository) CountByAssignee(ctx context.Context, projectID string) ([]AssigneeCount, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT assignee_id, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY assignee_id`, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AssigneeCount
	for rows.Next() {
		var ac AssigneeCount
		if err := rows.Scan(&ac.AssigneeID, &ac.Count); err != nil {
			return nil, err
		}
		results = append(results, ac)
	}
	return results, rows.Err()
}
