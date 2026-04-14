package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/taskflow/backend/internal/model"
)

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

func (r *ProjectRepository) Create(ctx context.Context, p *model.Project) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO projects (id, name, description, owner_id, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		p.ID, p.Name, p.Description, p.OwnerID, p.CreatedAt,
	)
	return err
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (*model.Project, error) {
	var p model.Project
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, owner_id, created_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepository) FindByUser(ctx context.Context, userID string, limit, offset int) ([]model.Project, int, error) {
	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT p.id) FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id
		 WHERE p.owner_id = $1 OR t.assignee_id = $1 OR t.creator_id = $1`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
		 FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id
		 WHERE p.owner_id = $1 OR t.assignee_id = $1 OR t.creator_id = $1
		 ORDER BY p.created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		projects = append(projects, p)
	}
	return projects, total, rows.Err()
}

func (r *ProjectRepository) Update(ctx context.Context, id string, name *string, desc *string) (*model.Project, error) {
	setClauses := ""
	args := []interface{}{}
	argIdx := 1

	if name != nil {
		setClauses += fmt.Sprintf("name = $%d, ", argIdx)
		args = append(args, *name)
		argIdx++
	}
	if desc != nil {
		setClauses += fmt.Sprintf("description = $%d, ", argIdx)
		args = append(args, *desc)
		argIdx++
	}

	if setClauses == "" {
		return r.FindByID(ctx, id)
	}

	setClauses = setClauses[:len(setClauses)-2]

	query := fmt.Sprintf(
		`UPDATE projects SET %s WHERE id = $%d RETURNING id, name, description, owner_id, created_at`,
		setClauses, argIdx,
	)
	args = append(args, id)

	var p model.Project
	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id, ownerID string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM projects WHERE id = $1 AND owner_id = $2`, id, ownerID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *ProjectRepository) IsUserInProject(ctx context.Context, projectID, userID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM projects p
			LEFT JOIN tasks t ON t.project_id = p.id
			WHERE p.id = $1 AND (p.owner_id = $2 OR t.assignee_id = $2 OR t.creator_id = $2)
		)`, projectID, userID,
	).Scan(&exists)
	return exists, err
}
