package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
)

type ugenTemplateRepo struct {
	db *Pool
}

func NewUgenTemplateRepo(db *Pool) repo.UgenTemplateStorageI {
	return &ugenTemplateRepo{db: db}
}

func (r *ugenTemplateRepo) Create(ctx context.Context, req *pb.CreateUgenTemplateReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.Create")
	defer dbSpan.Finish()

	id := uuid.New().String()
	// price and currency_id are intentionally not set here: templates are created
	// for free by users and default to price 0. Only the admin SetPrice endpoint
	// assigns a price. They are still read back so the response is complete.
	stmt := `INSERT INTO ugen_template (
				id, name, description, photo, mcp_project_id, preview_url,
				source_resource_env_id, source_project_id, source_environment_id, source_node_type,
				source_mcp_resource_env_id, source_function_id, source_repo_id, images,
				order_number
			 )
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			 RETURNING id, name, description, photo, mcp_project_id, preview_url,
				source_resource_env_id, source_project_id, source_environment_id, source_node_type,
				source_mcp_resource_env_id, source_function_id, source_repo_id, images,
				0::int, 0::int, order_number, created_at, updated_at, deleted_at,
				COALESCE(price, 0), COALESCE(currency_id::text, ''), COALESCE(per_user_price, 0)`

	var resp pb.UgenTemplate
	var createdAt, updatedAt time.Time
	var deletedAt int64
	var likeCount, dislikeCount int32

	err := r.db.QueryRow(ctx, stmt,
		id, req.Name, req.Description, req.Photo, req.McpProjectId, req.PreviewUrl,
		req.SourceResourceEnvId, req.SourceProjectId, req.SourceEnvironmentId, req.SourceNodeType,
		req.SourceMcpResourceEnvId, req.SourceFunctionId, req.SourceRepoId, req.Images,
		req.OrderNumber,
	).Scan(
		&resp.Id, &resp.Name, &resp.Description, &resp.Photo, &resp.McpProjectId, &resp.PreviewUrl,
		&resp.SourceResourceEnvId, &resp.SourceProjectId, &resp.SourceEnvironmentId, &resp.SourceNodeType,
		&resp.SourceMcpResourceEnvId, &resp.SourceFunctionId, &resp.SourceRepoId, &resp.Images,
		&likeCount, &dislikeCount, &resp.OrderNumber,
		&createdAt, &updatedAt, &deletedAt,
		&resp.Price, &resp.CurrencyId, &resp.PerUserPrice,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ugen_template: %w", err)
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	resp.DeletedAt = deletedAt
	setUgenTemplateReactionFields(&resp, likeCount, dislikeCount, pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED)
	return &resp, nil
}

func (r *ugenTemplateRepo) GetById(ctx context.Context, id, userID string) (*pb.UgenTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.GetById")
	defer dbSpan.Finish()

	stmt := `SELECT id, name, COALESCE(description, ''), COALESCE(photo, ''), COALESCE(mcp_project_id, ''), COALESCE(preview_url, ''),
				COALESCE(source_resource_env_id, ''), COALESCE(source_project_id, ''), COALESCE(source_environment_id, ''), COALESCE(source_node_type, ''),
				COALESCE(source_mcp_resource_env_id, ''), COALESCE(source_function_id, ''), COALESCE(source_repo_id, ''), COALESCE(images, '{}'),
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = ugen_template.id AND r.reaction_type = 'like' AND r.deleted_at = 0),
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = ugen_template.id AND r.reaction_type = 'dislike' AND r.deleted_at = 0),
				COALESCE((
					SELECT r.reaction_type FROM ugen_template_reaction r
					WHERE r.ugen_template_id = ugen_template.id AND r.user_id = $2 AND r.deleted_at = 0
					LIMIT 1
				), ''),
				order_number, created_at, updated_at, deleted_at,
				COALESCE(price, 0), COALESCE(currency_id::text, ''), COALESCE(per_user_price, 0)
			 FROM ugen_template WHERE id = $1 AND deleted_at = 0`

	var resp pb.UgenTemplate
	var createdAt, updatedAt time.Time
	var deletedAt int64
	var likeCount, dislikeCount int32
	var currentReaction string

	err := r.db.QueryRow(ctx, stmt, id, userID).Scan(
		&resp.Id, &resp.Name, &resp.Description, &resp.Photo, &resp.McpProjectId, &resp.PreviewUrl,
		&resp.SourceResourceEnvId, &resp.SourceProjectId, &resp.SourceEnvironmentId, &resp.SourceNodeType,
		&resp.SourceMcpResourceEnvId, &resp.SourceFunctionId, &resp.SourceRepoId, &resp.Images,
		&likeCount, &dislikeCount, &currentReaction, &resp.OrderNumber,
		&createdAt, &updatedAt, &deletedAt,
		&resp.Price, &resp.CurrencyId, &resp.PerUserPrice,
	)
	if err != nil {
		if isUgenTemplateNotFoundErr(err) {
			return nil, fmt.Errorf("ugen_template not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get ugen_template: %w", err)
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	resp.DeletedAt = deletedAt
	setUgenTemplateReactionFields(&resp, likeCount, dislikeCount, ugenTemplateReactionTypeFromDB(currentReaction))
	return &resp, nil
}

func (r *ugenTemplateRepo) GetList(ctx context.Context, req *pb.GetUgenTemplateListReq) (*pb.GetUgenTemplateListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.GetList")
	defer dbSpan.Finish()

	resp := &pb.GetUgenTemplateListResponse{}

	countStmt := `SELECT COUNT(*) FROM ugen_template WHERE deleted_at = 0`
	if err := r.db.QueryRow(ctx, countStmt).Scan(&resp.Count); err != nil {
		return nil, fmt.Errorf("failed to count ugen_template: %w", err)
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	stmt := `SELECT id, name, COALESCE(description, ''), COALESCE(photo, ''), COALESCE(mcp_project_id, ''), COALESCE(preview_url, ''),
				COALESCE(source_resource_env_id, ''), COALESCE(source_project_id, ''), COALESCE(source_environment_id, ''), COALESCE(source_node_type, ''),
				COALESCE(source_mcp_resource_env_id, ''), COALESCE(source_function_id, ''), COALESCE(source_repo_id, ''), COALESCE(images, '{}'),
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = ugen_template.id AND r.reaction_type = 'like' AND r.deleted_at = 0),
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = ugen_template.id AND r.reaction_type = 'dislike' AND r.deleted_at = 0),
				COALESCE((
					SELECT r.reaction_type FROM ugen_template_reaction r
					WHERE r.ugen_template_id = ugen_template.id AND r.user_id = $3 AND r.deleted_at = 0
					LIMIT 1
				), ''),
				order_number, created_at, updated_at, deleted_at,
				COALESCE(price, 0), COALESCE(currency_id::text, ''), COALESCE(per_user_price, 0)
			 FROM ugen_template WHERE deleted_at = 0
			 ORDER BY order_number ASC, created_at DESC
			 LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, stmt, req.Limit, req.Offset, req.GetUserId())
	if err != nil {
		return nil, fmt.Errorf("failed to list ugen_template: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t pb.UgenTemplate
		var createdAt, updatedAt time.Time
		var deletedAt int64
		var likeCount, dislikeCount int32
		var currentReaction string

		if err := rows.Scan(
			&t.Id, &t.Name, &t.Description, &t.Photo, &t.McpProjectId, &t.PreviewUrl,
			&t.SourceResourceEnvId, &t.SourceProjectId, &t.SourceEnvironmentId, &t.SourceNodeType,
			&t.SourceMcpResourceEnvId, &t.SourceFunctionId, &t.SourceRepoId, &t.Images,
			&likeCount, &dislikeCount, &currentReaction, &t.OrderNumber,
			&createdAt, &updatedAt, &deletedAt,
			&t.Price, &t.CurrencyId, &t.PerUserPrice,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ugen_template: %w", err)
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		t.DeletedAt = deletedAt
		setUgenTemplateReactionFields(&t, likeCount, dislikeCount, ugenTemplateReactionTypeFromDB(currentReaction))
		resp.Templates = append(resp.Templates, &t)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ugen_template rows: %w", err)
	}

	return resp, nil
}

func (r *ugenTemplateRepo) Update(ctx context.Context, req *pb.UpdateUgenTemplateReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.Update")
	defer dbSpan.Finish()

	stmt := `WITH updated AS (
				UPDATE ugen_template SET
					name = $2, description = $3, photo = $4, mcp_project_id = $5, preview_url = $6,
					source_resource_env_id = $7, source_project_id = $8, source_environment_id = $9, source_node_type = $10,
					source_mcp_resource_env_id = $11, source_function_id = $12, source_repo_id = $13, images = $14,
					order_number = $15,
					updated_at = CURRENT_TIMESTAMP
				 WHERE id = $1 AND deleted_at = 0
				 RETURNING id, name, description, photo, mcp_project_id, preview_url,
					source_resource_env_id, source_project_id, source_environment_id, source_node_type,
					source_mcp_resource_env_id, source_function_id, source_repo_id, images,
					order_number, created_at, updated_at, deleted_at, price, currency_id, per_user_price
			 )
			 SELECT id, name, description, photo, mcp_project_id, preview_url,
				source_resource_env_id, source_project_id, source_environment_id, source_node_type,
				source_mcp_resource_env_id, source_function_id, source_repo_id, images,
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = updated.id AND r.reaction_type = 'like' AND r.deleted_at = 0),
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = updated.id AND r.reaction_type = 'dislike' AND r.deleted_at = 0),
				order_number, created_at, updated_at, deleted_at,
				COALESCE(price, 0), COALESCE(currency_id::text, ''), COALESCE(per_user_price, 0)
			 FROM updated`

	var resp pb.UgenTemplate
	var createdAt, updatedAt time.Time
	var deletedAt int64
	var likeCount, dislikeCount int32

	err := r.db.QueryRow(ctx, stmt,
		req.Id, req.Name, req.Description, req.Photo, req.McpProjectId, req.PreviewUrl,
		req.SourceResourceEnvId, req.SourceProjectId, req.SourceEnvironmentId, req.SourceNodeType,
		req.SourceMcpResourceEnvId, req.SourceFunctionId, req.SourceRepoId, req.Images,
		req.OrderNumber,
	).Scan(
		&resp.Id, &resp.Name, &resp.Description, &resp.Photo, &resp.McpProjectId, &resp.PreviewUrl,
		&resp.SourceResourceEnvId, &resp.SourceProjectId, &resp.SourceEnvironmentId, &resp.SourceNodeType,
		&resp.SourceMcpResourceEnvId, &resp.SourceFunctionId, &resp.SourceRepoId, &resp.Images,
		&likeCount, &dislikeCount, &resp.OrderNumber,
		&createdAt, &updatedAt, &deletedAt,
		&resp.Price, &resp.CurrencyId, &resp.PerUserPrice,
	)
	if err != nil {
		if isUgenTemplateNotFoundErr(err) {
			return nil, fmt.Errorf("ugen_template not found or deleted: %s", req.Id)
		}
		return nil, fmt.Errorf("failed to update ugen_template: %w", err)
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	resp.DeletedAt = deletedAt
	setUgenTemplateReactionFields(&resp, likeCount, dislikeCount, pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED)
	return &resp, nil
}

// SetPrice assigns (or updates) a template's price and currency. This is the
// admin-only price control; Create/Update never touch price, so a template stays
// free (price 0) until an admin calls this. Passing price 0 makes it free again.
func (r *ugenTemplateRepo) SetPrice(ctx context.Context, req *pb.SetUgenTemplatePriceReq) (*pb.UgenTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.SetPrice")
	defer dbSpan.Finish()

	stmt := `WITH updated AS (
				UPDATE ugen_template SET
					price = $2, currency_id = NULLIF($3, '')::uuid, per_user_price = $4,
					updated_at = CURRENT_TIMESTAMP
				 WHERE id = $1 AND deleted_at = 0
				 RETURNING id, name, description, photo, mcp_project_id, preview_url,
					source_resource_env_id, source_project_id, source_environment_id, source_node_type,
					source_mcp_resource_env_id, source_function_id, source_repo_id, images,
					order_number, created_at, updated_at, deleted_at, price, currency_id, per_user_price
			 )
			 SELECT id, name, description, photo, mcp_project_id, preview_url,
				source_resource_env_id, source_project_id, source_environment_id, source_node_type,
				source_mcp_resource_env_id, source_function_id, source_repo_id, images,
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = updated.id AND r.reaction_type = 'like' AND r.deleted_at = 0),
				(SELECT COUNT(*)::int FROM ugen_template_reaction r WHERE r.ugen_template_id = updated.id AND r.reaction_type = 'dislike' AND r.deleted_at = 0),
				order_number, created_at, updated_at, deleted_at,
				COALESCE(price, 0), COALESCE(currency_id::text, ''), COALESCE(per_user_price, 0)
			 FROM updated`

	var resp pb.UgenTemplate
	var createdAt, updatedAt time.Time
	var deletedAt int64
	var likeCount, dislikeCount int32

	err := r.db.QueryRow(ctx, stmt, req.GetId(), req.GetPrice(), req.GetCurrencyId(), req.GetPerUserPrice()).Scan(
		&resp.Id, &resp.Name, &resp.Description, &resp.Photo, &resp.McpProjectId, &resp.PreviewUrl,
		&resp.SourceResourceEnvId, &resp.SourceProjectId, &resp.SourceEnvironmentId, &resp.SourceNodeType,
		&resp.SourceMcpResourceEnvId, &resp.SourceFunctionId, &resp.SourceRepoId, &resp.Images,
		&likeCount, &dislikeCount, &resp.OrderNumber,
		&createdAt, &updatedAt, &deletedAt,
		&resp.Price, &resp.CurrencyId, &resp.PerUserPrice,
	)
	if err != nil {
		if isUgenTemplateNotFoundErr(err) {
			return nil, fmt.Errorf("ugen_template not found or deleted: %s", req.GetId())
		}
		return nil, fmt.Errorf("failed to set ugen_template price: %w", err)
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	resp.DeletedAt = deletedAt
	setUgenTemplateReactionFields(&resp, likeCount, dislikeCount, pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED)
	return &resp, nil
}

func (r *ugenTemplateRepo) Delete(ctx context.Context, id string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.Delete")
	defer dbSpan.Finish()

	stmt := `UPDATE ugen_template SET
				deleted_at = date_part('epoch', CURRENT_TIMESTAMP)::int,
				updated_at = CURRENT_TIMESTAMP
			 WHERE id = $1 AND deleted_at = 0`

	result, err := r.db.Exec(ctx, stmt, id)
	if err != nil {
		return fmt.Errorf("failed to delete ugen_template: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("ugen_template not found or already deleted: %s", id)
	}
	return nil
}

func (r *ugenTemplateRepo) SetReaction(ctx context.Context, req *pb.SetUgenTemplateReactionReq) (*pb.UgenTemplateReaction, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.SetReaction")
	defer dbSpan.Finish()

	reactionType, err := ugenTemplateReactionTypeToDB(req.GetReactionType())
	if err != nil {
		return nil, err
	}

	stmt := `INSERT INTO ugen_template_reaction (
				id, ugen_template_id, user_id, reaction_type
			 )
			 SELECT $1, id, $3, $4
			 FROM ugen_template
			 WHERE id = $2 AND deleted_at = 0
			 ON CONFLICT (ugen_template_id, user_id) WHERE deleted_at = 0
			 DO UPDATE SET
				reaction_type = EXCLUDED.reaction_type,
				updated_at = CURRENT_TIMESTAMP
			 RETURNING id, ugen_template_id, user_id, reaction_type, created_at, updated_at, deleted_at`

	reactionID := uuid.New().String()
	var reaction pb.UgenTemplateReaction
	var createdAt, updatedAt time.Time
	var deletedAt int64
	var reactionTypeDB string

	err = r.db.QueryRow(ctx, stmt,
		reactionID, req.GetTemplateId(), req.GetUserId(), reactionType,
	).Scan(
		&reaction.Id, &reaction.TemplateId, &reaction.UserId, &reactionTypeDB,
		&createdAt, &updatedAt, &deletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set ugen_template reaction: %w", err)
	}

	reaction.ReactionType = ugenTemplateReactionTypeFromDB(reactionTypeDB)
	reaction.CreatedAt = createdAt.Format(time.RFC3339)
	reaction.UpdatedAt = updatedAt.Format(time.RFC3339)
	reaction.DeletedAt = deletedAt
	return &reaction, nil
}

func (r *ugenTemplateRepo) DeleteReaction(ctx context.Context, req *pb.DeleteUgenTemplateReactionReq) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.DeleteReaction")
	defer dbSpan.Finish()

	stmt := `UPDATE ugen_template_reaction SET
				deleted_at = date_part('epoch', CURRENT_TIMESTAMP)::int,
				updated_at = CURRENT_TIMESTAMP
			 WHERE ugen_template_id = $1 AND user_id = $2 AND deleted_at = 0`

	result, err := r.db.Exec(ctx, stmt, req.GetTemplateId(), req.GetUserId())
	if err != nil {
		return fmt.Errorf("failed to delete ugen_template reaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("ugen_template reaction not found")
	}
	return nil
}

func (r *ugenTemplateRepo) ListReactions(ctx context.Context, req *pb.GetUgenTemplateReactionListReq) (*pb.GetUgenTemplateReactionListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "ugen_template.ListReactions")
	defer dbSpan.Finish()

	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	reactionType, err := ugenTemplateOptionalReactionTypeToDB(req.GetReactionType())
	if err != nil {
		return nil, err
	}

	resp := &pb.GetUgenTemplateReactionListResponse{}
	countStmt := `SELECT COUNT(*) FROM ugen_template_reaction
		WHERE ugen_template_id = $1 AND deleted_at = 0 AND ($2 = '' OR reaction_type = $2)`
	if err = r.db.QueryRow(ctx, countStmt, req.GetTemplateId(), reactionType).Scan(&resp.Count); err != nil {
		return nil, fmt.Errorf("failed to count ugen_template reactions: %w", err)
	}

	stmt := `SELECT id, ugen_template_id, user_id, reaction_type, created_at, updated_at, deleted_at
		FROM ugen_template_reaction
		WHERE ugen_template_id = $1 AND deleted_at = 0 AND ($2 = '' OR reaction_type = $2)
		ORDER BY updated_at DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.db.Query(ctx, stmt, req.GetTemplateId(), reactionType, req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, fmt.Errorf("failed to list ugen_template reactions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var reaction pb.UgenTemplateReaction
		var reactionTypeDB string
		var createdAt, updatedAt time.Time
		var deletedAt int64

		if err = rows.Scan(
			&reaction.Id, &reaction.TemplateId, &reaction.UserId, &reactionTypeDB,
			&createdAt, &updatedAt, &deletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ugen_template reaction: %w", err)
		}
		reaction.ReactionType = ugenTemplateReactionTypeFromDB(reactionTypeDB)
		reaction.CreatedAt = createdAt.Format(time.RFC3339)
		reaction.UpdatedAt = updatedAt.Format(time.RFC3339)
		reaction.DeletedAt = deletedAt
		resp.Reactions = append(resp.Reactions, &reaction)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ugen_template reaction rows: %w", err)
	}

	return resp, nil
}

func setUgenTemplateReactionFields(t *pb.UgenTemplate, likeCount, dislikeCount int32, currentReaction pb.UgenTemplateReactionType) {
	if t == nil {
		return
	}
	t.LikeCount = likeCount
	t.DislikeCount = dislikeCount
	t.CurrentUserReaction = currentReaction
}

func ugenTemplateReactionTypeToDB(reactionType pb.UgenTemplateReactionType) (string, error) {
	switch reactionType {
	case pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_LIKE:
		return "like", nil
	case pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_DISLIKE:
		return "dislike", nil
	default:
		return "", fmt.Errorf("reaction_type must be like or dislike")
	}
}

func ugenTemplateOptionalReactionTypeToDB(reactionType pb.UgenTemplateReactionType) (string, error) {
	if reactionType == pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED {
		return "", nil
	}
	return ugenTemplateReactionTypeToDB(reactionType)
}

func ugenTemplateReactionTypeFromDB(reactionType string) pb.UgenTemplateReactionType {
	switch reactionType {
	case "like":
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_LIKE
	case "dislike":
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_DISLIKE
	default:
		return pb.UgenTemplateReactionType_UGEN_TEMPLATE_REACTION_TYPE_UNSPECIFIED
	}
}

func isUgenTemplateNotFoundErr(err error) bool {
	return err == sql.ErrNoRows || err == pgx.ErrNoRows
}
