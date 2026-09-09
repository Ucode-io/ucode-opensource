package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/util"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/structpb"
)

type mcpProjectRepo struct {
	db *psqlpool.Pool
}

func NewMcpProjectRepo(db *psqlpool.Pool) storage.McpProjectRepoI {
	return &mcpProjectRepo{
		db: db,
	}
}

func (m *mcpProjectRepo) CreateMcpProject(ctx context.Context, req *nb.CreateMcpProjectReqeust) (*nb.McpProject, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.CreateMcpProject")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if req.AppVisibility == "" {
		req.AppVisibility = "public"
	}

	var (
		projectId  = uuid.NewString()
		now        = time.Now()
		projectEnv = map[string]any{}

		projectQuery = `
			INSERT INTO mcp_project (
				id, title, description, project_env, ucode_project_id, api_key, environment_id, status,
				app_visibility, is_published, project_image, microfrontend_id, microfrontend_repo_id,
				microfrontend_branch, microfrontend_url, project_type, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, $10, $11, $12, $13, $14, $15, $16, $16)`
	)

	if req.GetProjectEnv() != nil {
		projectEnv = req.GetProjectEnv().AsMap()
	}

	_, err = tx.Exec(ctx, projectQuery, projectId, req.GetTitle(), req.GetDescription(), projectEnv,
		nullString(req.GetUcodeProjectId()),
		nullString(req.GetApiKey()),
		nullString(req.GetEnvironmentId()),
		nullString(req.GetStatus()),
		strings.ToLower(req.GetAppVisibility()),
		req.GetProjectImage(),
		nullString(req.GetMicrofrontendId()),
		nullString(req.GetMicrofrontendRepoId()),
		nullString(req.GetMicrofrontendBranch()),
		nullString(req.GetMicrofrontendUrl()),
		nullString(strings.ToLower(req.GetProjectType())),
		now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert mcp_project: %w", err)
	}

	if len(req.GetProjectFiles()) > 0 {
		var (
			valueStrings = make([]string, 0, len(req.GetProjectFiles()))
			valueArgs    = make([]interface{}, 0, len(req.GetProjectFiles())*7)

			argIndex = 1
		)

		for _, file := range req.GetProjectFiles() {
			var (
				fileId       = uuid.NewString()
				fileGraphMap = file.GetFileGraph().AsMap()
			)

			valueStrings = append(valueStrings, fmt.Sprintf(
				"($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6,
			))

			valueArgs = append(valueArgs,
				fileId,
				projectId,
				file.GetPath(),
				file.GetContent(),
				fileGraphMap,
				now,
				now,
			)

			argIndex += 7
		}

		fileQuery := fmt.Sprintf(`
			INSERT INTO project_files (id, project_id, file_path, content, file_graph, created_at, updated_at)
			VALUES %s
		`, strings.Join(valueStrings, ", "))

		_, err = tx.Exec(ctx, fileQuery, valueArgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to insert project_files: %w", err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	result, err := m.GetMcpProjectFiles(ctx,
		&nb.McpProjectId{
			ResourceEnvId: req.GetResourceEnvId(),
			Id:            projectId,
		},
	)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *mcpProjectRepo) UpdateMcpProject(ctx context.Context, req *nb.McpProject) (*nb.McpProject, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.UpdateMcpProject")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err = m.updateProjectFields(ctx, tx, req); err != nil {
		return nil, err
	}

	if err = m.upsertProjectFiles(ctx, tx, req.GetId(), req.GetProjectFiles()); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return m.GetMcpProjectFiles(ctx, &nb.McpProjectId{
		ResourceEnvId: req.GetResourceEnvId(),
		Id:            req.GetId(),
	})
}

func (m *mcpProjectRepo) updateProjectFields(ctx context.Context, tx pgx.Tx, req *nb.McpProject) error {
	var (
		setClauses = []string{"updated_at = $1"}
		args       = []any{time.Now()}
		argIndex   = 2
	)

	if req.GetTitle() != "" {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIndex))
		args = append(args, req.GetTitle())
		argIndex++
	}

	if req.GetDescription() != "" {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, req.GetDescription())
		argIndex++
	}

	if req.GetProjectEnv() != nil {
		setClauses = append(setClauses, fmt.Sprintf("project_env = $%d", argIndex))
		args = append(args, req.GetProjectEnv().AsMap())
		argIndex++
	}

	if util.IsValidUUID(req.GetFunctionId()) {
		setClauses = append(setClauses, fmt.Sprintf("function_id = $%d", argIndex))
		args = append(args, req.GetFunctionId())
		argIndex++
	}

	if req.GetUcodeProjectId() != "" {
		setClauses = append(setClauses, fmt.Sprintf("ucode_project_id = $%d", argIndex))
		args = append(args, req.GetUcodeProjectId())
		argIndex++
	}

	if req.GetApiKey() != "" {
		setClauses = append(setClauses, fmt.Sprintf("api_key = $%d", argIndex))
		args = append(args, req.GetApiKey())
		argIndex++
	}

	if req.GetEnvironmentId() != "" {
		setClauses = append(setClauses, fmt.Sprintf("environment_id = $%d", argIndex))
		args = append(args, req.GetEnvironmentId())
		argIndex++
	}

	if req.GetStatus() != "" {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, req.GetStatus())
		argIndex++
	}

	if req.GetAppVisibility() != "" {
		setClauses = append(setClauses, fmt.Sprintf("app_visibility = $%d", argIndex))
		args = append(args, strings.ToLower(req.GetAppVisibility()))
		argIndex++
	}

	if req.GetIsPublished() {
		setClauses = append(setClauses, fmt.Sprintf("is_published = $%d", argIndex))
		args = append(args, req.GetIsPublished())
		argIndex++
	}

	if req.GetProjectImage() != "" {
		setClauses = append(setClauses, fmt.Sprintf("project_image = $%d", argIndex))
		args = append(args, req.GetProjectImage())
		argIndex++
	}

	if req.GetMicrofrontendId() != "" {
		setClauses = append(setClauses, fmt.Sprintf("microfrontend_id = $%d", argIndex))
		args = append(args, req.GetMicrofrontendId())
		argIndex++
	}

	if req.GetMicrofrontendRepoId() != "" {
		setClauses = append(setClauses, fmt.Sprintf("microfrontend_repo_id = $%d", argIndex))
		args = append(args, req.GetMicrofrontendRepoId())
		argIndex++
	}

	if req.GetMicrofrontendBranch() != "" {
		setClauses = append(setClauses, fmt.Sprintf("microfrontend_branch = $%d", argIndex))
		args = append(args, req.GetMicrofrontendBranch())
		argIndex++
	}

	if req.GetMicrofrontendUrl() != "" {
		setClauses = append(setClauses, fmt.Sprintf("microfrontend_url = $%d", argIndex))
		args = append(args, req.GetMicrofrontendUrl())
		argIndex++
	}

	if req.GetProjectType() != "" {
		setClauses = append(setClauses, fmt.Sprintf("project_type = $%d", argIndex))
		args = append(args, strings.ToLower(req.GetProjectType()))
		argIndex++
	}

	args = append(args, req.GetId())

	var query = fmt.Sprintf(`
		UPDATE mcp_project
		SET %s
		WHERE id = $%d
	`, strings.Join(setClauses, ", "), argIndex)

	_, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update mcp_project: %w", err)
	}

	return nil
}

func (m *mcpProjectRepo) upsertProjectFiles(ctx context.Context, tx pgx.Tx, projectId string, files []*nb.McpProjectFiles) error {
	if len(files) == 0 {
		return nil
	}

	var (
		ids        []string
		filePaths  []string
		contents   []string
		fileGraphs []map[string]interface{}
	)

	for _, file := range files {
		fileId := file.GetId()
		if fileId == "" {
			fileId = uuid.NewString()
		}

		ids = append(ids, fileId)
		filePaths = append(filePaths, file.GetPath())
		contents = append(contents, file.GetContent())
		fileGraphs = append(fileGraphs, file.GetFileGraph().AsMap())
	}

	var query = `
       INSERT INTO project_files (id, project_id, file_path, content, file_graph, created_at, updated_at)
       SELECT 
          unnest($1::uuid[]),
          $2::uuid,
          unnest($3::text[]),
          unnest($4::text[]),
          unnest($5::jsonb[]),
          $6::timestamp,
          $6::timestamp
       
       ON CONFLICT (project_id, file_path) WHERE deleted_at IS NULL
       
       DO UPDATE SET
          content = EXCLUDED.content,
          file_graph = EXCLUDED.file_graph,
          updated_at = EXCLUDED.updated_at
    `

	_, err := tx.Exec(ctx, query,
		ids,
		projectId,
		filePaths,
		contents,
		fileGraphs,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to upsert project_files: %w", err)
	}

	return nil
}

func (m *mcpProjectRepo) GetAllMcpProject(ctx context.Context, req *nb.GetMcpProjectListReq) (*nb.McpProjectList, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.GetAllMcpProject")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	var (
		queryBuilder strings.Builder
		args         = make([]any, 0)
		projects     = make([]*nb.McpProject, 0)

		orderDir    = "DESC"
		orderColumn = "mp.created_at"
	)

	queryBuilder.WriteString(`
        SELECT mp.id, mp.title, mp.description, mp.project_env, mp.ucode_project_id, mp.api_key, mp.environment_id, mp.status, mp.app_visibility, mp.is_published, mp.project_image,
               mp.microfrontend_id, mp.microfrontend_repo_id, mp.microfrontend_branch, mp.microfrontend_url, mp.project_type,
               mp.created_at, mp.updated_at,
               f.id, f.name, f.path, f.type, f.url, f.branch, f.repo_id,
               f.created_at, f.updated_at
        FROM mcp_project AS mp
        LEFT JOIN function AS f ON mp.function_id = f.id
        WHERE mp.is_published = true
    `)

	if len(req.GetIds()) > 0 {
		args = append(args, req.GetIds())
		queryBuilder.WriteString(fmt.Sprintf(" AND mp.id = ANY($%d::uuid[])", len(args)))
	}

	if req.GetTitle() != "" {
		args = append(args, "%"+req.GetTitle()+"%")
		queryBuilder.WriteString(fmt.Sprintf(" AND mp.title ILIKE $%d", len(args)))
	}

	if col, ok := config.McProjectAllowedOrder[req.GetOrderBy()]; ok {
		orderColumn = col
	}

	if req.GetOrderDirection() == "asc" {
		orderDir = "ASC"
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderColumn, orderDir))

	args = append(args, req.GetLimit(), req.GetOffset())
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)))

	rows, err := conn.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return &nb.McpProjectList{}, nil
		}
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var (
			project              nb.McpProject
			projectEnv           map[string]any
			createdAt, updatedAt time.Time

			ucodeProjectId, apiKey, environmentId, status, appVisibility                sql.NullString
			microfrontendId, microfrontendRepoId, microfrontendBranch, microfrontendUrl sql.NullString
			projectType                                                                 sql.NullString
			fId, fName, fPath, fType, fUrl, fBranch, fRepoId                            sql.NullString
			fCreatedAt, fUpdatedAt                                                      sql.NullTime
		)

		project.FunctionData = &nb.FunctionData{}

		err = rows.Scan(
			&project.Id, &project.Title, &project.Description, &projectEnv, &ucodeProjectId, &apiKey, &environmentId, &status, &appVisibility, &project.IsPublished, &project.ProjectImage,
			&microfrontendId, &microfrontendRepoId, &microfrontendBranch, &microfrontendUrl, &projectType, &createdAt, &updatedAt,
			&fId, &fName, &fPath, &fType, &fUrl, &fBranch, &fRepoId, &fCreatedAt, &fUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if ucodeProjectId.Valid {
			project.UcodeProjectId = ucodeProjectId.String
		}
		if apiKey.Valid {
			project.ApiKey = apiKey.String
		}
		if environmentId.Valid {
			project.EnvironmentId = environmentId.String
		}
		if status.Valid {
			project.Status = status.String
		}
		if appVisibility.Valid {
			project.AppVisibility = appVisibility.String
		}
		if microfrontendId.Valid {
			project.MicrofrontendId = microfrontendId.String
		}
		if microfrontendRepoId.Valid {
			project.MicrofrontendRepoId = microfrontendRepoId.String
		}
		if microfrontendBranch.Valid {
			project.MicrofrontendBranch = microfrontendBranch.String
		}
		if microfrontendUrl.Valid {
			project.MicrofrontendUrl = microfrontendUrl.String
		}
		if projectType.Valid {
			project.ProjectType = projectType.String
		}

		project.CreatedAt = createdAt.Format(time.RFC3339)
		project.UpdatedAt = updatedAt.Format(time.RFC3339)

		if projectEnv != nil {
			envStruct, _ := structpb.NewStruct(projectEnv)
			project.ProjectEnv = envStruct
		}

		if fId.Valid {
			project.FunctionId = fId.String
			project.FunctionData.Id = fId.String
			project.FunctionData.Name = fName.String
			project.FunctionData.Path = fPath.String
			project.FunctionData.Type = fType.String
			if project.GetIsPublished() {
				project.FunctionData.Url = fUrl.String
			}
			project.FunctionData.Branch = fBranch.String
			project.FunctionData.RepoId = fRepoId.String
			if fCreatedAt.Valid {
				project.FunctionData.CreatedAt = fCreatedAt.Time.Format(time.RFC3339)
			}
			if fUpdatedAt.Valid {
				project.FunctionData.UpdatedAt = fUpdatedAt.Time.Format(time.RFC3339)
			}
		} else if project.GetMicrofrontendId() != "" || project.GetMicrofrontendRepoId() != "" {
			project.FunctionData.Id = project.GetMicrofrontendId()
			project.FunctionData.Branch = project.GetMicrofrontendBranch()
			project.FunctionData.RepoId = project.GetMicrofrontendRepoId()
			if project.GetIsPublished() {
				project.FunctionData.Url = project.GetMicrofrontendUrl()
			}
		}

		projects = append(projects, &project)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return &nb.McpProjectList{
		Projects: projects,
	}, nil
}

func (m *mcpProjectRepo) GetMcpProjectFiles(ctx context.Context, req *nb.McpProjectId) (*nb.McpProject, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.GetMcpProjectFiles")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	var (
		project                nb.McpProject
		projectEnv             map[string]any
		pCreatedAt, pUpdatedAt time.Time

		fId, fName, fPath, fType, fUrl, fBranch, fRepoId sql.NullString
		fCreatedAt, fUpdatedAt                           sql.NullTime

		projectQuery = `
			SELECT
				mp.id, mp.title, mp.description, mp.project_env, mp.ucode_project_id, mp.api_key, mp.environment_id, mp.status, mp.app_visibility, mp.is_published, mp.project_image,
				mp.microfrontend_id, mp.microfrontend_repo_id, mp.microfrontend_branch, mp.microfrontend_url, mp.project_type,
				mp.created_at, mp.updated_at,
				f.id, f.name, f.path, f.type, f.url, f.branch, f.repo_id, f.created_at, f.updated_at
			FROM mcp_project mp
			LEFT JOIN function f ON mp.function_id = f.id
			WHERE mp.id = $1
    `
	)

	var ucodeProjectId, apiKey, environmentId, pStatus, appVisibility sql.NullString
	var microfrontendId, microfrontendRepoId, microfrontendBranch, microfrontendUrl sql.NullString
	var projectType sql.NullString

	err = conn.QueryRow(ctx, projectQuery, req.GetId()).Scan(
		&project.Id, &project.Title, &project.Description, &projectEnv, &ucodeProjectId, &apiKey, &environmentId, &pStatus, &appVisibility, &project.IsPublished, &project.ProjectImage,
		&microfrontendId, &microfrontendRepoId, &microfrontendBranch, &microfrontendUrl, &projectType, &pCreatedAt, &pUpdatedAt,
		&fId, &fName, &fPath, &fType, &fUrl, &fBranch, &fRepoId, &fCreatedAt, &fUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("project not found: %s", req.GetId())
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	project.CreatedAt = pCreatedAt.Format(time.RFC3339)
	project.UpdatedAt = pUpdatedAt.Format(time.RFC3339)
	project.ResourceEnvId = req.GetResourceEnvId()

	if ucodeProjectId.Valid {
		project.UcodeProjectId = ucodeProjectId.String
	}
	if apiKey.Valid {
		project.ApiKey = apiKey.String
	}
	if environmentId.Valid {
		project.EnvironmentId = environmentId.String
	}
	if pStatus.Valid {
		project.Status = pStatus.String
	}
	if appVisibility.Valid {
		project.AppVisibility = appVisibility.String
	}
	if microfrontendId.Valid {
		project.MicrofrontendId = microfrontendId.String
	}
	if microfrontendRepoId.Valid {
		project.MicrofrontendRepoId = microfrontendRepoId.String
	}
	if microfrontendBranch.Valid {
		project.MicrofrontendBranch = microfrontendBranch.String
	}
	if microfrontendUrl.Valid {
		project.MicrofrontendUrl = microfrontendUrl.String
	}
	if projectType.Valid {
		project.ProjectType = projectType.String
	}

	if projectEnv != nil {
		project.ProjectEnv, _ = structpb.NewStruct(projectEnv)
	}

	if fId.Valid {
		project.FunctionId = fId.String
		project.FunctionData = &nb.FunctionData{
			Id:     fId.String,
			Name:   fName.String,
			Path:   fPath.String,
			Type:   fType.String,
			Branch: fBranch.String,
			RepoId: fRepoId.String,
		}
		if project.GetIsPublished() {
			project.FunctionData.Url = fUrl.String
		}
		if fCreatedAt.Valid {
			project.FunctionData.CreatedAt = fCreatedAt.Time.Format(time.RFC3339)
		}
		if fUpdatedAt.Valid {
			project.FunctionData.UpdatedAt = fUpdatedAt.Time.Format(time.RFC3339)
		}
	} else if project.GetMicrofrontendId() != "" || project.GetMicrofrontendRepoId() != "" {
		project.FunctionData = &nb.FunctionData{
			Id:     project.GetMicrofrontendId(),
			Branch: project.GetMicrofrontendBranch(),
			RepoId: project.GetMicrofrontendRepoId(),
		}
		if project.GetIsPublished() {
			project.FunctionData.Url = project.GetMicrofrontendUrl()
		}
	}

	if !req.GetWithoutFiles() {
		var filesQuery = `
            SELECT id, project_id, file_path, content, file_graph, created_at, updated_at
            FROM project_files
            WHERE project_id = $1
            ORDER BY created_at ASC
        `
		rows, err := conn.Query(ctx, filesQuery, req.GetId())
		if err != nil {
			return nil, fmt.Errorf("failed to query project files: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var (
				file                 nb.McpProjectFiles
				fileGraph            map[string]any
				createdAt, updatedAt time.Time
			)

			err = rows.Scan(
				&file.Id, &file.ProjectId, &file.Path, &file.Content, &fileGraph, &createdAt, &updatedAt,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to scan project file: %w", err)
			}

			file.CreatedAt = createdAt.Format(time.RFC3339)
			file.UpdatedAt = updatedAt.Format(time.RFC3339)
			file.ResourceEnvId = req.GetResourceEnvId()

			if fileGraph != nil {
				file.FileGraph, _ = structpb.NewStruct(fileGraph)
			}

			project.ProjectFiles = append(project.ProjectFiles, &file)
		}

		if err = rows.Err(); err != nil {
			return nil, fmt.Errorf("rows iteration error: %w", err)
		}
	}

	return &project, nil
}

func (m *mcpProjectRepo) GetPublishedMcpProjectCount(ctx context.Context, req *nb.GetPublishedMcpProjectCountReq) (*nb.GetPublishedMcpProjectCountResp, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.GetPublishedMcpProjectCount")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	var count int32
	if err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM mcp_project WHERE is_published = true`).Scan(&count); err != nil {
		return nil, fmt.Errorf("failed to count published mcp projects: %w", err)
	}

	return &nb.GetPublishedMcpProjectCountResp{Count: count}, nil
}

func (m *mcpProjectRepo) DeleteMcpProject(ctx context.Context, req *nb.McpProjectId) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.DeleteMcpProject")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var deleteFilesQuery = `DELETE FROM project_files WHERE project_id = $1`
	_, err = tx.Exec(ctx, deleteFilesQuery, req.GetId())
	if err != nil {
		return fmt.Errorf("failed to delete project files: %w", err)
	}

	var deleteProjectQuery = `DELETE FROM mcp_project WHERE id = $1`
	_, err = tx.Exec(ctx, deleteProjectQuery, req.GetId())
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

//func (m *mcpProjectRepo) DeleteProjectFile(ctx context.Context, req *nb.McpProjectId) error {
//	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mcp_project.DeleteProjectFile")
//	defer dbSpan.Finish()
//
//	conn, err := psqlpool.Get(req.GetResourceEnvId())
//	if err != nil {
//		return err
//	}
//
//	query := `DELETE FROM project_files WHERE project_id = $1`
//	args := []interface{}{req.GetProjectId()}
//	argIndex := 2
//
//	// Можем удалять либо по file_id, либо по file_path
//	if req.GetFileId() != "" {
//		query += fmt.Sprintf(" AND id = $%d", argIndex)
//		args = append(args, req.GetFileId())
//	} else if req.GetPath() != "" {
//		query += fmt.Sprintf(" AND file_path = $%d", argIndex)
//		args = append(args, req.GetPath())
//	} else {
//		return fmt.Errorf("either file_id or file_path must be provided")
//	}
//
//	_, err = conn.Exec(ctx, query, args...)
//	if err != nil {
//		return fmt.Errorf("failed to delete project file: %w", err)
//	}
//
//	return nil
//}
