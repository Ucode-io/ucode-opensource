package postgres

import (
	"context"
	"database/sql"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

func (b *billingRepo) CreateCard(ctx context.Context, req *pb.CreateProjectCardRequest) (*pb.ProjectCard, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.CreateCard")
	defer dbSpan.Finish()

	projectId := req.ProjectId

	_, err := uuid.Parse(projectId)
	if err != nil {
		query := `SELECT id FROM project WHERE customer_id = $1`
		err = b.db.QueryRow(ctx, query, req.ProjectId).Scan(&projectId)
		if err != nil {
			return nil, errors.Wrap(err, "error getting project by customer id")
		}
	}

	query := `
		INSERT INTO "project_card" ("id", "pan", "expire", "payme_token", "project_id", "type", "external_id", "verify")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (pan, project_id) DO UPDATE SET
			payme_token = EXCLUDED.payme_token
		RETURNING "id", "pan", "expire", "payme_token", "project_id", "type", "external_id", "verify"
	`
	var createdCard pb.ProjectCard
	err = b.db.QueryRow(ctx, query,
		uuid.NewString(),
		req.Pan,
		req.Expire,
		req.PaymeToken,
		projectId,
		req.Type,
		req.ExternalId,
		req.Verify,
	).Scan(
		&createdCard.Id,
		&createdCard.Pan,
		&createdCard.Expire,
		&createdCard.PaymeToken,
		&createdCard.ProjectId,
		&createdCard.Type,
		&createdCard.ExternalId,
		&createdCard.Verify,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error creating project card")
	}

	return &createdCard, nil
}

func (b *billingRepo) GetProjectCard(ctx context.Context, req *pb.PrimaryKey) (*pb.ProjectCard, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.GetProjectCard")
	defer dbSpan.Finish()

	query := `
		SELECT
			id,
			pan,
			expire,
			payme_token,
			project_id,
			verify,
			type,
			external_id
		FROM
			project_card
		WHERE
			id = $1
	`

	var projectCard pb.ProjectCard
	err := b.db.QueryRow(ctx, query, req.Id).Scan(
		&projectCard.Id,
		&projectCard.Pan,
		&projectCard.Expire,
		&projectCard.PaymeToken,
		&projectCard.ProjectId,
		&projectCard.Verify,
		&projectCard.Type,
		&projectCard.ExternalId,
	)

	if err != nil {
		return nil, errors.Wrap(err, "error selecting project card")
	}

	return &projectCard, nil
}

func (b *billingRepo) UpdateProjectCard(ctx context.Context, req *pb.ProjectCard) (*pb.ProjectCard, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.UpdateProjectCard")
	defer dbSpan.Finish()

	query := `
		UPDATE project_card
		SET
			pan = $1,
			expire = $2,
			project_id = $3,
			verify = $4,
			updated_at = CURRENT_TIMESTAMP
		WHERE
			id = $5
		RETURNING
			id,
			pan,
			expire,
			project_id,
			verify
	`

	var updatedProjectCard pb.ProjectCard
	err := b.db.QueryRow(ctx, query,
		req.Pan,
		req.Expire,
		req.ProjectId,
		req.Verify,
		req.Id,
	).Scan(
		&updatedProjectCard.Id,
		&updatedProjectCard.Pan,
		&updatedProjectCard.Expire,
		&updatedProjectCard.ProjectId,
		&updatedProjectCard.Verify,
	)

	if err != nil {
		return nil, errors.Wrap(err, "error updating project card")
	}

	return &updatedProjectCard, nil
}

func (b *billingRepo) ListProjectCards(ctx context.Context, req *pb.ListRequest) (*pb.ListProjectCardsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.ListProjectCards")
	defer dbSpan.Finish()

	var (
		response = &pb.ListProjectCardsResponse{}
		count    int32
	)

	query := `
		SELECT 
			COUNT(*) OVER() AS total_count,
			id, 
			pan, 
			expire,
			verify, 
			project_id,
			type,
			external_id
		FROM project_card 
		WHERE project_id = $1 AND verify = TRUE
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := b.db.Query(ctx, query, req.ProjectId, req.Limit, req.Offset)
	if err != nil {
		return nil, errors.Wrap(err, "error getting project cards")
	}
	defer rows.Close()

	var projectCards []*pb.ProjectCard
	for rows.Next() {
		var externalId sql.NullString
		card := &pb.ProjectCard{}
		if err := rows.Scan(
			&count,
			&card.Id,
			&card.Pan,
			&card.Expire,
			&card.Verify,
			&card.ProjectId,
			&card.Type,
			&externalId,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan project card")
		}

		card.ExternalId = externalId.String

		projectCards = append(projectCards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	response.ProjectCards = projectCards
	response.Count = count

	return response, nil
}

func (b *billingRepo) DeleteProjectCard(ctx context.Context, req *pb.PrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "billing.DeleteProjectCard")
	defer dbSpan.Finish()

	query := `DELETE FROM project_card WHERE id = $1`

	result, err := b.db.Exec(ctx, query, req.Id)
	if err != nil {
		return errors.Wrap(err, "failed to delete project card")
	}

	if result.RowsAffected() == 0 {
		return errors.New("no project card with the given id")
	}

	return nil
}
