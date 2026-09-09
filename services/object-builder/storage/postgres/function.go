package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
)

type functionRepo struct {
	db *psqlpool.Pool
}

func NewFunctionRepo(db *psqlpool.Pool) storage.FunctionRepoI {
	return &functionRepo{
		db: db,
	}
}

func (f functionRepo) Create(ctx context.Context, req *nb.CreateFunctionRequest) (resp *nb.Function, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "function.Create")
	defer dbSpan.Finish()

	var (
		functionId = uuid.NewString()
		query      = `INSERT INTO "function" (
				id,
				name,
				path,
				type,
				description,
				project_id,
				environment_id,
				url,
				password,
				ssh_url,
				gitlab_id,
				gitlab_group_id,
				request_time,
				source_url,
				branch,
				error_message,
				pipeline_status,
				repo_id,
				is_public,
				mcp_project_id,
				mcp_resource_env_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(ctx, query,
		functionId,
		req.Name,
		req.Path,
		req.Type,
		req.Description,
		req.ProjectId,
		req.EnvironmentId,
		req.Url,
		req.Password,
		req.SshUrl,
		req.GitlabId,
		req.GitlabGroupId,
		time.Now().Format(time.RFC3339),
		req.SourceUrl,
		req.Branch,
		req.ErrorMessage,
		req.PipelineStatus,
		req.RepoId,
		req.IsPublic,
		nullString(req.GetMcpProjectId()),
		nullString(req.GetMcpResourceEnvId()),
	)
	if err != nil {
		return &nb.Function{}, err
	}

	return f.GetSingle(ctx, &nb.FunctionPrimaryKey{Id: functionId, ProjectId: req.ProjectId})
}

func (f *functionRepo) GetList(ctx context.Context, req *nb.GetAllFunctionsRequest) (resp *nb.GetAllFunctionsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "function.GetList")
	defer dbSpan.Finish()
	resp = &nb.GetAllFunctionsResponse{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT
		id,
		name,
		path,
		type,
		description,
		project_id,
		environment_id,
		COALESCE(url, ''),
		COALESCE(branch, ''),
		COALESCE(source_url, ''),
		COALESCE(error_message, ''),
		COALESCE(pipeline_status, ''),
		is_public,
		max_scale,
		COALESCE(repo_id, ''),
		COALESCE(github_repo_name, ''),
		COALESCE(github_webhook_id, ''),
		COALESCE(mcp_project_id::text, ''),
		COALESCE(mcp_resource_env_id::text, '')
	FROM "function" WHERE deleted_at IS NULL`

	var args []any
	argIndex := 1

	if len(req.FunctionId) > 0 {
		query += fmt.Sprintf(` OR id = $%d`, argIndex)
		args = append(args, req.FunctionId)
		argIndex++
	}

	if len(req.Type) > 0 {
		query += fmt.Sprintf(` AND type = ANY($%d)`, argIndex)
		args = append(args, pq.Array(req.Type))
		argIndex++
	}

	if req.Search != "" {
		query += fmt.Sprintf(` AND name ~* $%d`, argIndex)
		args = append(args, req.Search)
		argIndex++
	}

	if req.GithubRepoName != "" {
		query += fmt.Sprintf(` AND github_repo_name = $%d`, argIndex)
		args = append(args, req.GithubRepoName)
		argIndex++
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.GetAllFunctionsResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := &nb.Function{}

		var (
			name         sql.NullString
			path         sql.NullString
			functionType sql.NullString
			desc         sql.NullString
			projectId    sql.NullString
			envId        sql.NullString
		)

		err = rows.Scan(
			&row.Id,
			&name,
			&path,
			&functionType,
			&desc,
			&projectId,
			&envId,
			&row.Url,
			&row.Branch,
			&row.SourceUrl,
			&row.ErrorMessage,
			&row.PipelineStatus,
			&row.IsPublic,
			&row.MaxScale,
			&row.RepoId,
			&row.GithubRepoName,
			&row.GithubWebhookId,
			&row.McpProjectId,
			&row.McpResourceEnvId,
		)
		if err != nil {
			return &nb.GetAllFunctionsResponse{}, err
		}

		row.Name = name.String
		row.Path = path.String
		row.Type = functionType.String
		row.Description = desc.String
		row.ProjectId = projectId.String
		row.EnvironmentId = envId.String

		resp.Functions = append(resp.Functions, row)
	}

	if err = rows.Err(); err != nil {
		return &nb.GetAllFunctionsResponse{}, err
	}

	//THIS LOGIC ADDED FOR UGEN
	if req.GetIncludeCustomEvents() {
		if err = f.attachCustomEvents(ctx, conn, resp.Functions); err != nil {
			return &nb.GetAllFunctionsResponse{}, err
		}
	}

	return resp, nil
}

// THIS FUNCTION ADDED FOR UGEN
func (f *functionRepo) attachCustomEvents(ctx context.Context, conn *psqlpool.Pool, functions []*nb.Function) error {
	for _, function := range functions {
		function.CustomEvents = []*nb.FunctionCustomEvent{}
	}

	if len(functions) == 0 {
		return nil
	}

	functionIDs := make([]string, 0, len(functions))
	for _, function := range functions {
		functionIDs = append(functionIDs, function.GetId())
	}

	query := `
	SELECT
		id::text,
		table_slug,
		COALESCE(event_path::text, ''),
		label,
		COALESCE(icon, ''),
		COALESCE(url, ''),
		COALESCE(disable, false),
		COALESCE(method, ''),
		COALESCE(action_type, ''),
		COALESCE(attributes, '{}'::jsonb),
		COALESCE(path, '')
	FROM custom_event
	WHERE deleted_at IS NULL
		AND event_path::text = ANY($1::text[])
	ORDER BY created_at ASC`

	rows, err := conn.Query(ctx, query, pq.Array(functionIDs))
	if err != nil {
		return err
	}
	defer rows.Close()

	events := make([]*nb.FunctionCustomEvent, 0)
	for rows.Next() {
		var (
			event      = nb.FunctionCustomEvent{}
			attrsBytes []byte
			attrsMap   map[string]any
		)

		if err = rows.Scan(
			&event.Id,
			&event.TableSlug,
			&event.EventPath,
			&event.Label,
			&event.Icon,
			&event.Url,
			&event.Disable,
			&event.Method,
			&event.ActionType,
			&attrsBytes,
			&event.Path,
		); err != nil {
			return err
		}

		if len(attrsBytes) > 0 {
			if err = json.Unmarshal(attrsBytes, &attrsMap); err != nil {
				return err
			}
		}

		if attrsMap == nil {
			attrsMap = map[string]any{}
		}

		event.Attributes, err = helper.ConvertMapToStruct(attrsMap)
		if err != nil {
			return err
		}

		events = append(events, &event)
	}

	if err = rows.Err(); err != nil {
		return err
	}

	groupFunctionCustomEvents(functions, events)

	return nil
}

func groupFunctionCustomEvents(functions []*nb.Function, events []*nb.FunctionCustomEvent) {
	eventsByFunctionID := make(map[string][]*nb.FunctionCustomEvent, len(functions))
	for _, event := range events {
		eventsByFunctionID[event.GetEventPath()] = append(eventsByFunctionID[event.GetEventPath()], event)
	}

	for _, function := range functions {
		function.CustomEvents = eventsByFunctionID[function.GetId()]
		if function.CustomEvents == nil {
			function.CustomEvents = []*nb.FunctionCustomEvent{}
		}
	}
}

func (f *functionRepo) GetSingle(ctx context.Context, req *nb.FunctionPrimaryKey) (resp *nb.Function, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "function.GetSingle")
	defer dbSpan.Finish()
	resp = &nb.Function{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var (
		name              sql.NullString
		path              sql.NullString
		functionType      sql.NullString
		desc, repoId      sql.NullString
		projectId         sql.NullString
		envId, url        sql.NullString
		branch, sourceUrl sql.NullString
		functionFolderId  sql.NullString
		filter            string
		args              = []any{}
	)

	query := `SELECT
		id,
		name,
		path,
		type,
		description,
		project_id,
		environment_id,
		function_folder_id,
		url,
		branch,
		source_url,
		repo_id,
		is_public,
		max_scale,
		COALESCE(github_repo_name, ''),
		COALESCE(github_webhook_id, ''),
		COALESCE(mcp_project_id::text, ''),
		COALESCE(mcp_resource_env_id::text, '')
	FROM "function" WHERE `

	if req.Id != "" {
		filter = "id = $1"
		args = append(args, req.Id)
	} else if req.Path != "" {
		filter = "path = $1"
		args = append(args, req.Path)
	} else if req.SourceUrl != "" && req.Branch != "" {
		filter = "source_url = $1 AND branch = $2"
		args = append(args, req.SourceUrl, req.Branch)
	} else if req.RepoId != "" {
		filter = "repo_id = $1"
		args = append(args, req.RepoId)
	} else {
		return resp, fmt.Errorf("one of id, path, source_url+branch, or repo_id is required")
	}

	query += filter

	err = conn.QueryRow(ctx, query, args...).Scan(
		&resp.Id,
		&name,
		&path,
		&functionType,
		&desc,
		&projectId,
		&envId,
		&functionFolderId,
		&url,
		&branch,
		&sourceUrl,
		&repoId,
		&resp.IsPublic,
		&resp.MaxScale,
		&resp.GithubRepoName,
		&resp.GithubWebhookId,
		&resp.McpProjectId,
		&resp.McpResourceEnvId,
	)
	if err != nil {
		return resp, err
	}

	resp.Name = name.String
	resp.Path = path.String
	resp.Type = functionType.String
	resp.Description = desc.String
	resp.ProjectId = projectId.String
	resp.EnvironmentId = envId.String
	resp.FunctionFolderId = functionFolderId.String
	resp.Url = url.String
	resp.Branch = branch.String
	resp.SourceUrl = sourceUrl.String
	resp.RepoId = repoId.String

	return resp, nil
}

func (f *functionRepo) Update(ctx context.Context, req *nb.Function) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "function.Update")
	defer dbSpan.Finish()
	var (
		query = `UPDATE "function" SET
					name = $2,
					path = $3,
					type = $4,
					description = $5,
					project_id = $6,
					environment_id = $7,
					url = $8,
					password = $9,
					ssh_url = $10,
					gitlab_id = $11,
					gitlab_group_id = $12,
					error_message = $13,
					pipeline_status = $14,
					is_public = $15,
					max_scale = $16,
					github_repo_name = $17,
					github_webhook_id = $18,
					mcp_project_id = COALESCE(NULLIF($19, '')::uuid, mcp_project_id),
					mcp_resource_env_id = COALESCE(NULLIF($20, '')::uuid, mcp_resource_env_id)
				WHERE id = $1
	`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	_, err = conn.Exec(ctx, query,
		req.Id,
		req.Name,
		req.Path,
		req.Type,
		req.Description,
		req.ProjectId,
		req.EnvironmentId,
		req.Url,
		req.Password,
		req.SshUrl,
		req.GitlabId,
		req.GitlabGroupId,
		req.ErrorMessage,
		req.PipelineStatus,
		req.IsPublic,
		req.MaxScale,
		req.GithubRepoName,
		req.GithubWebhookId,
		req.McpProjectId,
		req.McpResourceEnvId,
	)
	if err != nil {
		return err
	}

	return nil
}

func (f *functionRepo) Delete(ctx context.Context, req *nb.FunctionPrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "function.Delete")
	defer dbSpan.Finish()
	var (
		query = `DELETE FROM "function" WHERE id = $1`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	_, err = conn.Exec(ctx, query, req.Id)
	if err != nil {
		return err
	}

	return nil
}

func (f *functionRepo) GetCountByType(ctx context.Context, req *nb.GetCountByTypeRequest) (*nb.GetCountByTypeResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "function.GetCountByType")
	defer dbSpan.Finish()

	var (
		query = `SELECT COUNT(*) FROM "function" WHERE type = ANY($1)`
		count int32
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	err = conn.QueryRow(ctx, query, pq.Array(req.Type)).Scan(&count)
	if err != nil {
		return &nb.GetCountByTypeResponse{}, err
	}

	return &nb.GetCountByTypeResponse{Count: count}, nil
}
