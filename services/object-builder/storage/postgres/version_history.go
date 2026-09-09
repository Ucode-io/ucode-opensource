package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cast"
)

var versionHistoryPartitionNameRe = regexp.MustCompile(`^version_history_p_\d{4}_\d{2}_\d{2}$`)

type versionHistoryRepo struct {
	db *psqlpool.Pool
}

func NewVersionHistoryRepo(db *psqlpool.Pool) storage.VersionHistoryRepoI {
	return &versionHistoryRepo{
		db: db,
	}
}

func (v *versionHistoryRepo) GetById(ctx context.Context, req *nb.VersionHistoryPrimaryKey) (*nb.VersionHistory, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.GetById")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			id,
			action_source,
			action_type,
			previous,
			current,
			date,
			user_info,
			request,
			response,
			api_key,
			type,
			table_slug,
			method_api,
			time_started,
			time_completed,
			duration,
			status_code,
			table_label
		FROM version_history
		WHERE id = $1
	`

	var (
		history       = &nb.VersionHistory{}
		methodApi     sql.NullString
		timeStarted   sql.NullString
		timeCompleted sql.NullString
		duration      sql.NullInt64
		statusCode    sql.NullInt64
		tableLabel    sql.NullString
	)
	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&history.Id,
		&history.ActionSource,
		&history.ActionType,
		&history.Previus,
		&history.Current,
		&history.Date,
		&history.UserInfo,
		&history.Request,
		&history.Response,
		&history.ApiKey,
		&history.Type,
		&history.TableSlug,
		&methodApi,
		&timeStarted,
		&timeCompleted,
		&duration,
		&statusCode,
		&tableLabel,
	)
	if err != nil {
		return &nb.VersionHistory{}, err
	}

	history.MethodApi = methodApi.String
	history.TimeStarted = timeStarted.String
	history.TimeCompleted = timeCompleted.String
	history.Duration = duration.Int64
	history.StatusCode = statusCode.Int64
	history.TableLabel = tableLabel.String

	return history, nil
}

func (v *versionHistoryRepo) GetAll(ctx context.Context, req *nb.GetAllRquest) (*nb.ListVersionHistory, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.GetAll")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	baseQuery := `
		FROM version_history WHERE true
	`
	args := []any{}
	argIndex := 1

	if req.Type == "DOWN" || req.Type == "UP" {
		baseQuery += fmt.Sprintf(" AND action_source IN (%s)", "'RELATION', 'FIELD', 'MENU', 'TABLE', 'LAYOUT', 'VIEW'")
	} else if req.Type != "" {
		baseQuery += fmt.Sprintf(" AND type = $%d", argIndex)
		args = append(args, req.Type)
		argIndex++
	}

	if req.FromDate != "" {
		baseQuery += fmt.Sprintf(" AND date >= $%d", argIndex)
		args = append(args, req.FromDate)
		argIndex++
	}
	if req.ToDate != "" {
		baseQuery += fmt.Sprintf(" AND date <= $%d", argIndex)
		args = append(args, req.ToDate)
		argIndex++
	}
	if req.UserInfo != "" {
		baseQuery += fmt.Sprintf(" AND user_info ILIKE $%d", argIndex)
		args = append(args, "%"+req.UserInfo+"%")
		argIndex++
	}
	if req.ApiKey != "" {
		baseQuery += fmt.Sprintf(" AND api_key ILIKE $%d", argIndex)
		args = append(args, "%"+req.ApiKey+"%")
		argIndex++
	}
	if req.ActionType != "" {
		baseQuery += fmt.Sprintf(" AND action_type ILIKE $%d", argIndex)
		args = append(args, "%"+req.ActionType+"%")
		argIndex++
	}
	if req.Collection != "" {
		baseQuery += fmt.Sprintf(" AND (table_slug ILIKE $%d OR table_label ILIKE $%d)", argIndex, argIndex)
		args = append(args, "%"+req.Collection+"%")
		argIndex++
	}

	// Query for fetching data
	sortOrder := "DESC"
	if req.OrderBy {
		sortOrder = "ASC"
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, action_source, action_type, previous, current, date, user_info, request, response, api_key, type, table_slug, method_api, time_started, time_completed, duration, status_code, table_label
		%s ORDER BY date %s LIMIT %d OFFSET %d`, baseQuery, sortOrder, req.Limit, req.Offset)

	rows, err := conn.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []*nb.VersionHistory
	for rows.Next() {
		var (
			history       nb.VersionHistory
			methodApi     sql.NullString
			timeStarted   sql.NullString
			timeCompleted sql.NullString
			duration      sql.NullInt64
			statusCode    sql.NullInt64
			tableLabel    sql.NullString
		)
		if err := rows.Scan(
			&history.Id,
			&history.ActionSource,
			&history.ActionType,
			&history.Previus,
			&history.Current,
			&history.Date,
			&history.UserInfo,
			&history.Request,
			&history.Response,
			&history.ApiKey,
			&history.Type,
			&history.TableSlug,
			&methodApi,
			&timeStarted,
			&timeCompleted,
			&duration,
			&statusCode,
			&tableLabel,
		); err != nil {
			return nil, err
		}

		history.MethodApi = methodApi.String
		history.TimeStarted = timeStarted.String
		history.TimeCompleted = timeCompleted.String
		history.Duration = duration.Int64
		history.StatusCode = statusCode.Int64
		history.TableLabel = tableLabel.String

		histories = append(histories, &history)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s", baseQuery)
	var count int32
	err = conn.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, err
	}

	return &nb.ListVersionHistory{
		Histories: histories,
		Count:     count,
	}, nil
}

func (v *versionHistoryRepo) Update(ctx context.Context, req *nb.UsedForEnvRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.Update")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `
		UPDATE object_builder_service.version_history
		SET used_environments = jsonb_set(
			used_environments, 
			ARRAY[$1], 
			'true'::jsonb, 
			true
		)
		WHERE id = ANY($2)
	`

	_, err = conn.Exec(ctx, query, req.EnvId, req.Ids)
	if err != nil {
		return err
	}

	return nil
}

func (v *versionHistoryRepo) Create(ctx context.Context, req *nb.CreateVersionHistoryRequest) (err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	versionH := `INSERT INTO version_history (
		action_source,
		action_type,
		previous,
		current,
		date,
		user_info,
		request,
		response,
		api_key,
		type,
		table_slug,
		method_api,
		time_started,
		time_completed,
		duration,
		status_code,
		table_label
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`

	labelQuery := `SELECT label FROM "table" `
	tableLabel := ""

	if err := uuid.Validate(req.TableSlug); err != nil {
		labelQuery += fmt.Sprintf(`WHERE slug = '%s'`, req.TableSlug)
	} else {
		labelQuery += fmt.Sprintf(`WHERE id = '%s'`, req.TableSlug)
	}

	err = conn.QueryRow(ctx, labelQuery).Scan(&tableLabel)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return err
	}

	if req.Type == "" {
		req.Type = "GLOBAL"
	}

	_, err = conn.Exec(ctx, versionH,
		req.ActionSource,
		req.ActionType,
		[]byte(req.Previus),
		[]byte(req.Current),
		req.Date,
		req.UserInfo,
		[]byte(req.Request),
		[]byte(req.Response),
		req.ApiKey,
		req.Type,
		req.TableSlug,
		req.MethodApi,
		req.TimeStarted,
		req.TimeCompleted,
		req.Duration,
		req.StatusCode,
		tableLabel,
	)
	if err != nil {
		return err
	}

	return nil
}

func (v *versionHistoryRepo) CreateFunctionLog(ctx context.Context, req *nb.FunctionLogReq) error {

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.CreateFunctionLog")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	if len(req.GetId()) == 0 {
		req.Id = uuid.NewString()
	}

	var query = ` INSERT INTO function_logs (id, function_id, table_slug, request_method,
			        action_type, send_at, completed_at, duration, compute, db_bandwidth,
			    	file_bandwidth, vector_bandwidth, return_size, status
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err = conn.Exec(ctx, query,
		req.Id, req.FunctionId, req.TableSlug, req.RequestMethod,
		req.ActionType, req.SendAt, req.CompletedAt,
		req.Duration, req.Compute, req.DbBandwidth,
		req.FileBandwidth, req.VectorBandwidth, req.ReturnSize, req.Status,
	)
	if err != nil {
		return err
	}

	return nil
}

func (v *versionHistoryRepo) GetFunctionLogs(ctx context.Context, req *nb.GetFunctionLogsReq) (*nb.GetFunctionLogsResp, error) {

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.GetFunctionLogs")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var (
		args  []any
		where = " WHERE 1=1 "

		query = `SELECT l.id, l.function_id, f.name, l.table_slug, l.request_method,l.action_type,
					l.send_at, l.completed_at, l.duration, l.compute, l.db_bandwidth,
			    	l.file_bandwidth, l.vector_bandwidth, l.return_size, l.status 
				FROM function_logs l
				    LEFT OUTER JOIN function f
				        ON l.function_id = f.id
				        `
	)

	if len(req.GetFunctionId()) > 0 {
		where += fmt.Sprintf(` AND l.function_id = $%d `, len(args)+1)
		args = append(args, req.GetFunctionId())
	}

	if len(req.GetTableSlug()) > 0 {
		where += fmt.Sprintf(` AND l.table_slug = $%d `, len(args)+1)
		args = append(args, req.GetTableSlug())
	}

	if len(req.GetRequestMethod()) > 0 {
		where += fmt.Sprintf(` AND l.request_method = $%d `, len(args)+1)
		args = append(args, req.GetRequestMethod())
	}

	if len(req.GetActionType()) > 0 {
		where += fmt.Sprintf(` AND l.action_type = $%d `, len(args)+1)
		args = append(args, req.GetActionType())
	}

	if len(req.GetStatus()) > 0 {
		where += fmt.Sprintf(` AND l.status = $%d `, len(args)+1)
		args = append(args, req.GetStatus())
	}

	if len(req.GetFromDate()) > 0 {
		where += fmt.Sprintf(` AND l.created_at >= $%d::timestamptz `, len(args)+1)
		args = append(args, req.GetFromDate())
	}

	if len(req.GetToDate()) > 0 {
		where += fmt.Sprintf(` AND l.created_at <= $%d::timestamptz `, len(args)+1)
		args = append(args, req.GetToDate())
	}

	if len(req.GetSearch()) > 0 {
		where += fmt.Sprintf(` AND (f.name ILIKE $%d OR l.table_slug ILIKE $%d OR l.request_method ILIKE $%d) `, len(args)+1, len(args)+1, len(args)+1)
		args = append(args, "%"+req.GetSearch()+"%")
	}

	var whereArgs = append([]any(nil), args...)

	query += fmt.Sprintf(` %s ORDER BY l.created_at DESC LIMIT $%d OFFSET $%d`, where, len(args)+1, len(args)+2)
	args = append(args, req.GetLimit(), req.GetOffset())

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var (
		functionId   any
		functionName any

		sendAt      any
		completedAt any

		response []*nb.FunctionLogModel
	)

	for rows.Next() {
		var respItem nb.FunctionLogModel
		err = rows.Scan(
			&respItem.Id,
			&functionId,
			&functionName,
			&respItem.TableSlug,
			&respItem.RequestMethod,
			&respItem.ActionType,
			&sendAt,
			&completedAt,
			&respItem.Duration,
			&respItem.Compute,
			&respItem.DbBandwidth,
			&respItem.FileBandwidth,
			&respItem.VectorBandwidth,
			&respItem.ReturnSize,
			&respItem.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		respItem.FunctionId = cast.ToString(functionId)
		respItem.FunctionName = cast.ToString(functionName)
		respItem.SendAt = cast.ToString(sendAt)
		respItem.CompletedAt = cast.ToString(completedAt)

		response = append(response, &respItem)
	}

	var (
		count      int64
		countQuery = fmt.Sprintf("SELECT COUNT(*) from function_logs AS l %s", where)
	)

	err = conn.QueryRow(ctx, countQuery, whereArgs...).Scan(&count)
	if err != nil {
		return nil, err
	}

	return &nb.GetFunctionLogsResp{
		FunctionLogs: response,
		TotalCount:   count,
	}, nil
}

func (v *versionHistoryRepo) DeleteFunctionLogs(ctx context.Context, projectId string) error {

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.GetFunctionLogs")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(projectId)
	if err != nil {
		return err
	}

	var (
		expireData = time.Now().AddDate(0, 0, -config.FUNCTIONS_LOG_EXPIERE_DAY)
		query      = `DELETE FROM function_logs WHERE created_at < $1`
	)

	_, err = conn.Exec(ctx, query, expireData)
	return err
}

// RotateVersionHistoryPartitions keeps the partitioned version_history table healthy:
//  1. ensures partitions exist for the current and next week,
//  2. drops partitions whose date range ends before the retention cutoff
//     (today - VERSION_HISTORY_EXPIRE_DAY days). DROP returns disk space to the OS immediately.
func (v *versionHistoryRepo) RotateVersionHistoryPartitions(ctx context.Context, projectId string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.RotateVersionHistoryPartitions")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(projectId)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	currentWeekStart := mondayOf(now)
	nextWeekStart := currentWeekStart.AddDate(0, 0, 7)
	cutoff := now.AddDate(0, 0, -config.VERSION_HISTORY_EXPIRE_DAY)

	for _, start := range []time.Time{currentWeekStart, nextWeekStart} {
		end := start.AddDate(0, 0, 7)
		partitionName := versionHistoryPartitionName(start)
		createSQL := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %q PARTITION OF "version_history" FOR VALUES FROM ('%s') TO ('%s')`,
			partitionName,
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)
		if _, err := conn.Exec(ctx, createSQL); err != nil {
			return fmt.Errorf("create partition %s: %w", partitionName, err)
		}
	}

	const listPartitionsQuery = `
		SELECT
			c.relname,
			(regexp_match(
				pg_get_expr(c.relpartbound, c.oid),
				'TO \(''(\d{4}-\d{2}-\d{2})''\)'
			))[1]::date AS upper_bound
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'version_history'::regclass
	`

	rows, err := conn.Query(ctx, listPartitionsQuery)
	if err != nil {
		return fmt.Errorf("list version_history partitions: %w", err)
	}
	defer rows.Close()

	type partitionRow struct {
		name       string
		upperBound time.Time
	}
	var toDrop []partitionRow
	for rows.Next() {
		var pr partitionRow
		if err := rows.Scan(&pr.name, &pr.upperBound); err != nil {
			return fmt.Errorf("scan partition row: %w", err)
		}
		// Drop only partitions that are fully past retention. upper_bound is exclusive.
		if !pr.upperBound.After(cutoff) {
			toDrop = append(toDrop, pr)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate partition rows: %w", err)
	}

	for _, pr := range toDrop {
		// Defence in depth: never drop anything that does not match our partition naming scheme.
		if !versionHistoryPartitionNameRe.MatchString(pr.name) {
			continue
		}
		dropSQL := fmt.Sprintf(`DROP TABLE IF EXISTS %q`, pr.name)
		if _, err := conn.Exec(ctx, dropSQL); err != nil {
			return fmt.Errorf("drop partition %s: %w", pr.name, err)
		}
	}

	return nil
}

func mondayOf(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // treat Sunday as end-of-week
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

func versionHistoryPartitionName(weekStart time.Time) string {
	return "version_history_p_" + weekStart.Format("2006_01_02")
}

func (v *versionHistoryRepo) GetPerformanceMetrics(ctx context.Context, req *nb.GetPerformanceMetricsRequest) (*nb.GetPerformanceMetricsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version_history.GetPerformanceMetrics")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			COALESCE(AVG(duration), 0),
			COALESCE(COUNT(*) FILTER (WHERE status_code >= 400)::float / NULLIF(COUNT(*), 0) * 100, 0)
		FROM version_history 
		WHERE 1=1
	`
	args := []any{}
	argIndex := 1

	if req.FromDate != "" {
		query += fmt.Sprintf(" AND date::date >= $%d", argIndex)
		args = append(args, req.FromDate)
		argIndex++
	}
	if req.ToDate != "" {
		query += fmt.Sprintf(" AND date::date <= $%d", argIndex)
		args = append(args, req.ToDate)
		argIndex++
	}

	var avgDuration float32
	var errorRate float32
	err = conn.QueryRow(ctx, query, args...).Scan(&avgDuration, &errorRate)
	if err != nil {
		return nil, err
	}

	return &nb.GetPerformanceMetricsResponse{
		AverageDuration: avgDuration,
		ErrorRate:       errorRate,
	}, nil
}
