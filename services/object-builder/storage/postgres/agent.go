package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/structpb"
)

const defaultAgentModel = "claude-sonnet-4-5"
const defaultAgentMaxSteps = 8

type agentRepo struct {
	db *psqlpool.Pool
}

func NewAgentRepo(db *psqlpool.Pool) storage.AgentRepoI {
	return &agentRepo{
		db: db,
	}
}

// ==================== Agents ====================

func (r *agentRepo) CreateAgent(ctx context.Context, req *nb.CreateAgentRequest) (*nb.Agent, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.CreateAgent")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		id  = uuid.NewString()
		now = time.Now()

		agent                nb.Agent
		createdAt, updatedAt time.Time

		model    = req.GetModel()
		maxSteps = req.GetMaxSteps()
	)

	if model == "" {
		model = defaultAgentModel
	}
	if maxSteps <= 0 {
		maxSteps = defaultAgentMaxSteps
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var query = `
		INSERT INTO agents (id, project_id, name, description, instruction, model, max_steps, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id, project_id, name, description, instruction, model, max_steps, enabled, created_at, updated_at
	`

	err = tx.QueryRow(ctx, query, id,
		req.GetProjectId(), req.GetName(), req.GetDescription(), req.GetInstruction(),
		model, maxSteps, req.GetEnabled(), now,
	).Scan(
		&agent.Id, &agent.ProjectId, &agent.Name, &agent.Description, &agent.Instruction,
		&agent.Model, &agent.MaxSteps, &agent.Enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	if err = insertAgentPermissions(ctx, tx, agent.Id, req.GetPermissions()); err != nil {
		return nil, fmt.Errorf("failed to insert agent permissions: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	agent.CreatedAt = createdAt.Format(time.RFC3339)
	agent.UpdatedAt = updatedAt.Format(time.RFC3339)

	agent.Permissions, err = getAgentPermissions(ctx, conn, agent.Id)
	if err != nil {
		return nil, err
	}

	return &agent, nil
}

func (r *agentRepo) GetAgentById(ctx context.Context, req *nb.AgentPrimaryKey) (*nb.Agent, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.GetAgentById")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		agent                nb.Agent
		createdAt, updatedAt time.Time

		query = `
			SELECT id, project_id, name, description, instruction, model, max_steps, enabled, created_at, updated_at
			FROM agents
			WHERE id = $1
		`
	)

	err = conn.QueryRow(ctx, query, req.GetId()).Scan(
		&agent.Id, &agent.ProjectId, &agent.Name, &agent.Description, &agent.Instruction,
		&agent.Model, &agent.MaxSteps, &agent.Enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("agent not found: %s", req.GetId())
		}
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	agent.CreatedAt = createdAt.Format(time.RFC3339)
	agent.UpdatedAt = updatedAt.Format(time.RFC3339)

	agent.Permissions, err = getAgentPermissions(ctx, conn, agent.Id)
	if err != nil {
		return nil, err
	}

	return &agent, nil
}

func (r *agentRepo) GetAllAgents(ctx context.Context, req *nb.GetAllAgentsRequest) (*nb.GetAllAgentsResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.GetAllAgents")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		queryBuilder strings.Builder
		countBuilder strings.Builder
		args         = make([]any, 0)
		agents       = make([]*nb.Agent, 0)
		agentIDs     = make([]string, 0)

		count       int32
		orderDir    = "DESC"
		orderColumn = "a.created_at"
	)

	queryBuilder.WriteString(`
		SELECT a.id, a.project_id, a.name, a.description, a.instruction, a.model, a.max_steps, a.enabled, a.created_at, a.updated_at
		FROM agents a
		WHERE 1=1
	`)
	countBuilder.WriteString(`SELECT COUNT(*) FROM agents a WHERE 1=1`)

	if req.GetProjectId() != "" {
		args = append(args, req.GetProjectId())
		queryBuilder.WriteString(fmt.Sprintf(" AND a.project_id = $%d", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND a.project_id = $%d", len(args)))
	}

	if req.GetName() != "" {
		args = append(args, "%"+req.GetName()+"%")
		queryBuilder.WriteString(fmt.Sprintf(" AND a.name ILIKE $%d", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND a.name ILIKE $%d", len(args)))
	}

	if req.GetModel() != "" {
		args = append(args, req.GetModel())
		queryBuilder.WriteString(fmt.Sprintf(" AND a.model = $%d", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND a.model = $%d", len(args)))
	}

	err = conn.QueryRow(ctx, countBuilder.String(), args...).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}

	if col, ok := config.AgentAllowedOrder[req.GetOrderBy()]; ok {
		orderColumn = col
	}

	if req.GetOrderDirection() == "asc" {
		orderDir = "ASC"
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderColumn, orderDir))

	if req.GetLimit() > 0 {
		args = append(args, req.GetLimit())
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	if req.GetOffset() > 0 {
		args = append(args, req.GetOffset())
		queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", len(args)))
	}

	rows, err := conn.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			agent                nb.Agent
			createdAt, updatedAt time.Time
		)

		err = rows.Scan(
			&agent.Id, &agent.ProjectId, &agent.Name, &agent.Description, &agent.Instruction,
			&agent.Model, &agent.MaxSteps, &agent.Enabled, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		agent.CreatedAt = createdAt.Format(time.RFC3339)
		agent.UpdatedAt = updatedAt.Format(time.RFC3339)

		agents = append(agents, &agent)
		agentIDs = append(agentIDs, agent.Id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	permissionsByAgent, err := getAgentPermissionsByAgentIds(ctx, conn, agentIDs)
	if err != nil {
		return nil, err
	}

	for _, agent := range agents {
		agent.Permissions = permissionsByAgent[agent.Id]
	}

	return &nb.GetAllAgentsResponse{
		Agents: agents,
		Count:  count,
	}, nil
}

// UpdateAgent is a full replace: every scalar is written and the permission set
// is dropped and re-inserted from req in a single transaction.
func (r *agentRepo) UpdateAgent(ctx context.Context, req *nb.UpdateAgentRequest) (*nb.Agent, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.UpdateAgent")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		agent                nb.Agent
		createdAt, updatedAt time.Time

		model    = req.GetModel()
		maxSteps = req.GetMaxSteps()
	)

	if model == "" {
		model = defaultAgentModel
	}
	if maxSteps <= 0 {
		maxSteps = defaultAgentMaxSteps
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var query = `
		UPDATE agents
		SET name = $2, description = $3, instruction = $4, model = $5, max_steps = $6, enabled = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING id, project_id, name, description, instruction, model, max_steps, enabled, created_at, updated_at
	`

	err = tx.QueryRow(ctx, query, req.GetId(),
		req.GetName(), req.GetDescription(), req.GetInstruction(),
		model, maxSteps, req.GetEnabled(),
	).Scan(
		&agent.Id, &agent.ProjectId, &agent.Name, &agent.Description, &agent.Instruction,
		&agent.Model, &agent.MaxSteps, &agent.Enabled, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("agent not found: %s", req.GetId())
		}
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM agent_permissions WHERE agent_id = $1`, agent.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to clear agent permissions: %w", err)
	}

	if err = insertAgentPermissions(ctx, tx, agent.Id, req.GetPermissions()); err != nil {
		return nil, fmt.Errorf("failed to insert agent permissions: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	agent.CreatedAt = createdAt.Format(time.RFC3339)
	agent.UpdatedAt = updatedAt.Format(time.RFC3339)

	agent.Permissions, err = getAgentPermissions(ctx, conn, agent.Id)
	if err != nil {
		return nil, err
	}

	return &agent, nil
}

func (r *agentRepo) DeleteAgent(ctx context.Context, req *nb.AgentPrimaryKey) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.DeleteAgent")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return err
	}

	var query = `DELETE FROM agents WHERE id = $1`
	res, err := conn.Exec(ctx, query, req.GetId())
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("agent not found: %s", req.GetId())
	}

	return nil
}

// ==================== Permissions ====================

func insertAgentPermissions(ctx context.Context, tx pgx.Tx, agentID string, perms []*nb.AgentPermission) error {
	if len(perms) == 0 {
		return nil
	}

	var (
		builder strings.Builder
		args    = make([]any, 0, len(perms)*7)
		argIdx  int
	)

	builder.WriteString(`
		INSERT INTO agent_permissions
			(agent_id, table_slug, can_create, can_read, can_update, can_delete, can_list)
		VALUES
	`)

	for i, p := range perms {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5, argIdx+6, argIdx+7))
		argIdx += 7
		args = append(args, agentID, p.GetTableSlug(),
			p.GetCanCreate(), p.GetCanRead(), p.GetCanUpdate(), p.GetCanDelete(), p.GetCanList())
	}

	_, err := tx.Exec(ctx, builder.String(), args...)
	return err
}

func getAgentPermissions(ctx context.Context, conn *psqlpool.Pool, agentID string) ([]*nb.AgentPermission, error) {
	permissions := make([]*nb.AgentPermission, 0)

	rows, err := conn.Query(ctx, `
		SELECT id, agent_id, table_slug, can_create, can_read, can_update, can_delete, can_list
		FROM agent_permissions
		WHERE agent_id = $1
		ORDER BY table_slug ASC
	`, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent permissions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p nb.AgentPermission
		err = rows.Scan(
			&p.Id, &p.AgentId, &p.TableSlug,
			&p.CanCreate, &p.CanRead, &p.CanUpdate, &p.CanDelete, &p.CanList,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent permission: %w", err)
		}
		permissions = append(permissions, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return permissions, nil
}

func getAgentPermissionsByAgentIds(ctx context.Context, conn *psqlpool.Pool, agentIDs []string) (map[string][]*nb.AgentPermission, error) {
	result := make(map[string][]*nb.AgentPermission)
	if len(agentIDs) == 0 {
		return result, nil
	}

	rows, err := conn.Query(ctx, `
		SELECT id, agent_id, table_slug, can_create, can_read, can_update, can_delete, can_list
		FROM agent_permissions
		WHERE agent_id = ANY($1)
		ORDER BY table_slug ASC
	`, agentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent permissions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p nb.AgentPermission
		err = rows.Scan(
			&p.Id, &p.AgentId, &p.TableSlug,
			&p.CanCreate, &p.CanRead, &p.CanUpdate, &p.CanDelete, &p.CanList,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent permission: %w", err)
		}
		result[p.AgentId] = append(result[p.AgentId], &p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

// ==================== Agent Runs ====================

func (r *agentRepo) CreateAgentRun(ctx context.Context, req *nb.CreateAgentRunRequest) (*nb.AgentRun, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.CreateAgentRun")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		id         = uuid.NewString()
		inputBytes = structToJSON(req.GetInput())

		query = `
			INSERT INTO agent_runs (id, agent_id, project_id, input)
			VALUES ($1, $2, $3, $4)
			RETURNING id, agent_id, project_id, status, input, output, steps, tokens_used, error, created_at, finished_at
		`
	)

	run, err := scanAgentRun(conn.QueryRow(ctx, query, id, req.GetAgentId(), req.GetProjectId(), inputBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create agent run: %w", err)
	}

	return run, nil
}

// UpdateAgentRun finalizes a run: it writes the terminal status, output,
// captured steps, token usage and error, and stamps finished_at server-side.
func (r *agentRepo) UpdateAgentRun(ctx context.Context, req *nb.UpdateAgentRunRequest) (*nb.AgentRun, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.UpdateAgentRun")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	stepsBytes, err := marshalSteps(req.GetSteps())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent run steps: %w", err)
	}

	var query = `
		UPDATE agent_runs
		SET status = $2::agent_run_status, output = $3, steps = $4, tokens_used = $5, error = $6, finished_at = NOW()
		WHERE id = $1
		RETURNING id, agent_id, project_id, status, input, output, steps, tokens_used, error, created_at, finished_at
	`

	run, err := scanAgentRun(conn.QueryRow(ctx, query, req.GetId(),
		req.GetStatus(), req.GetOutput(), stepsBytes, req.GetTokensUsed(), req.GetError(),
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("agent run not found: %s", req.GetId())
		}
		return nil, fmt.Errorf("failed to update agent run: %w", err)
	}

	return run, nil
}

func (r *agentRepo) GetAgentRunById(ctx context.Context, req *nb.AgentRunPrimaryKey) (*nb.AgentRun, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.GetAgentRunById")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var query = `
		SELECT id, agent_id, project_id, status, input, output, steps, tokens_used, error, created_at, finished_at
		FROM agent_runs
		WHERE id = $1
	`

	run, err := scanAgentRun(conn.QueryRow(ctx, query, req.GetId()))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("agent run not found: %s", req.GetId())
		}
		return nil, fmt.Errorf("failed to get agent run: %w", err)
	}

	return run, nil
}

func (r *agentRepo) GetAllAgentRuns(ctx context.Context, req *nb.GetAllAgentRunsRequest) (*nb.GetAllAgentRunsResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agentRepo.GetAllAgentRuns")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		queryBuilder strings.Builder
		countBuilder strings.Builder
		args         = make([]any, 0)
		runs         = make([]*nb.AgentRun, 0)

		count       int32
		orderDir    = "DESC"
		orderColumn = "created_at"
	)

	queryBuilder.WriteString(`
		SELECT id, agent_id, project_id, status, input, output, steps, tokens_used, error, created_at, finished_at
		FROM agent_runs
		WHERE 1=1
	`)
	countBuilder.WriteString(`SELECT COUNT(*) FROM agent_runs WHERE 1=1`)

	if req.GetAgentId() != "" {
		args = append(args, req.GetAgentId())
		queryBuilder.WriteString(fmt.Sprintf(" AND agent_id = $%d", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND agent_id = $%d", len(args)))
	}

	if req.GetStatus() != "" {
		args = append(args, req.GetStatus())
		queryBuilder.WriteString(fmt.Sprintf(" AND status = $%d::agent_run_status", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND status = $%d::agent_run_status", len(args)))
	}

	err = conn.QueryRow(ctx, countBuilder.String(), args...).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count agent runs: %w", err)
	}

	if col, ok := config.AgentRunAllowedOrder[req.GetOrderBy()]; ok {
		orderColumn = col
	}

	if req.GetOrderDirection() == "asc" {
		orderDir = "ASC"
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderColumn, orderDir))

	if req.GetLimit() > 0 {
		args = append(args, req.GetLimit())
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	if req.GetOffset() > 0 {
		args = append(args, req.GetOffset())
		queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", len(args)))
	}

	rows, err := conn.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent runs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		run, err := scanAgentRun(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent run: %w", err)
		}
		runs = append(runs, run)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return &nb.GetAllAgentRunsResponse{
		AgentRuns: runs,
		Count:     count,
	}, nil
}

func scanAgentRun(row rowScanner) (*nb.AgentRun, error) {
	var (
		run        nb.AgentRun
		inputBytes []byte
		stepsBytes []byte
		createdAt  time.Time
		finishedAt sql.NullTime
	)

	err := row.Scan(
		&run.Id, &run.AgentId, &run.ProjectId, &run.Status,
		&inputBytes, &run.Output, &stepsBytes, &run.TokensUsed, &run.Error,
		&createdAt, &finishedAt,
	)
	if err != nil {
		return nil, err
	}

	run.Input = jsonToStruct(inputBytes)

	run.Steps, err = unmarshalSteps(stepsBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent run steps: %w", err)
	}

	run.CreatedAt = createdAt.Format(time.RFC3339)
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time.Format(time.RFC3339)
	}

	return &run, nil
}

// ==================== JSONB helpers ====================

func structToJSON(s *structpb.Struct) []byte {
	if s == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(s.AsMap())
	if err != nil {
		return []byte("{}")
	}
	return b
}

func jsonToStruct(data []byte) *structpb.Struct {
	if len(data) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	s, err := structpb.NewStruct(raw)
	if err != nil {
		return nil
	}
	return s
}

func marshalSteps(steps []*nb.AgentRunStep) ([]byte, error) {
	raw := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		var toolInput map[string]any
		if step.GetToolInput() != nil {
			toolInput = step.GetToolInput().AsMap()
		}
		raw = append(raw, map[string]any{
			"index":       step.GetIndex(),
			"tool_name":   step.GetToolName(),
			"tool_input":  toolInput,
			"tool_result": step.GetToolResult(),
			"is_error":    step.GetIsError(),
		})
	}
	return json.Marshal(raw)
}

func unmarshalSteps(data []byte) ([]*nb.AgentRunStep, error) {
	steps := make([]*nb.AgentRunStep, 0)
	if len(data) == 0 {
		return steps, nil
	}

	var raw []struct {
		Index      int32          `json:"index"`
		ToolName   string         `json:"tool_name"`
		ToolInput  map[string]any `json:"tool_input"`
		ToolResult string         `json:"tool_result"`
		IsError    bool           `json:"is_error"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	for _, item := range raw {
		step := &nb.AgentRunStep{
			Index:      item.Index,
			ToolName:   item.ToolName,
			ToolResult: item.ToolResult,
			IsError:    item.IsError,
		}
		if item.ToolInput != nil {
			step.ToolInput, _ = structpb.NewStruct(item.ToolInput)
		}
		steps = append(steps, step)
	}

	return steps, nil
}
