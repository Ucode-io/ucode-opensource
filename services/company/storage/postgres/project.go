package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/models"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/emptypb"
)

type attachedSubscription struct {
	id                           string
	fareId                       string
	status                       string
	pendingFareId                string
	farePrice                    float64
	subType                      string
	startDate                    string
	endDate                      string
	billingPeriodCode            string
	billingPeriodMonths          int32
	billingPeriodDiscountPercent float64
	cancelAtPeriodEnd            bool
}

const (
	fareOpInitial         = "initial"
	fareOpUpgrade         = "upgrade"
	fareOpDowngrade       = "downgrade"
	fareOpRenewal         = "renewal"
	fareOpCancelDowngrade = "cancel_downgrade"
)

type projectRepo struct {
	db     *Pool
	logger logger.Logger
}

func NewProjectRepo(db *Pool, logger logger.Logger) repo.ProjectStorageI {
	return &projectRepo{
		db:     db,
		logger: logger,
	}
}

func (c *projectRepo) Create(ctx context.Context, id string, project *pb.CreateProjectRequest) (*pb.CreateProjectResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Create")
	defer dbSpan.Finish()

	var (
		result string
		err    error
	)

	if project.IsUgen {
		if err := c.validateUgenUniqueness(ctx, project.CompanyId); err != nil {
			return nil, err
		}
	}

	insertProject :=
		`INSERT INTO project(
			id,
			title,
			company_id,
			k8s_namespace,
			fare_id,
			is_ugen,
			per_user_price,
			per_user_currency_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, '')::uuid) RETURNING id`

	err = c.db.QueryRow(
		ctx,
		insertProject,
		id,
		project.Title,
		project.CompanyId,
		"cp-region-type-id", //@TODO:: use k8snamespace coming from request
		project.FareId,
		project.IsUgen,
		project.PerUserPrice,
		project.PerUserCurrencyId,
	).Scan(&result)

	if err != nil {
		return nil, err
	}

	return &pb.CreateProjectResponse{
		ProjectId: result,
	}, nil
}

func (c *projectRepo) GetById(ctx context.Context, projectId string) (*pb.Project, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetById")
	defer dbSpan.Finish()

	var (
		err                                        error
		resp                                       pb.Project
		lan                                        []*pb.Language
		tzone                                      pb.Timezone
		cur                                        pb.Currency
		lan_ids                                    []string
		currencies                                 []*pb.Currency
		fareId, currenciesString, subscriptionInfo sql.NullString
	)

	stmt :=
		`
			SELECT
				p.id,
				title,
				company_id,
				k8s_namespace,
				logo,
				coalesce(p.language_id, '{}'),
				coalesce(c.id::VARCHAR, ''),
				coalesce(c.symbol, ''),
				coalesce(c.name, ''),
				coalesce(c.symbol_native, ''),
				coalesce(c.decimal_digits, 0),
				coalesce(c.rounding, 0),
				coalesce(c.code, ''),
				coalesce(c.name_plural, ''),
				coalesce(t.id::VARCHAR, ''),
				coalesce(t.name, ''),
				coalesce(t.text, ''),
				fare_id,
				new_design,
				new_layout,
				new_router,
				status,
				balance,
				icon_categories,
				(SELECT json_agg(row_to_json(c)) FROM currency c) AS currencies,
				(
					SELECT json_build_object('end_date', s.end_date, 'type', s.type)
					FROM subscription s
					WHERE s.project_id = p.id
					ORDER BY s.created_at DESC
					LIMIT 1
				) AS subscription_info,
				customer_id,
				is_ugen,
				COALESCE(p.per_user_price, 0),
				COALESCE(p.per_user_currency_id::text, '')
			FROM project p
			LEFT JOIN currency c ON p.currency_id = c.id
			LEFT JOIN "timezone" t ON p.timezone_id = t.id
			WHERE p.id = $1;
		`

	err = c.db.QueryRow(ctx, stmt, projectId).Scan(
		&resp.ProjectId,
		&resp.Title,
		&resp.CompanyId,
		&resp.K8SNamespace,
		&resp.Logo,
		&lan_ids,
		&cur.Id,
		&cur.Symbol,
		&cur.Name,
		&cur.SymbolNative,
		&cur.DecimalDigits,
		&cur.Rounding,
		&cur.Code,
		&cur.NamePlural,
		&tzone.Id,
		&tzone.Name,
		&tzone.Text,
		&fareId,
		&resp.NewDesign,
		&resp.NewLayout,
		&resp.NewRouter,
		&resp.Status,
		&resp.Balance,
		&resp.IconCategories,
		&currenciesString,
		&subscriptionInfo,
		&resp.CustomerId,
		&resp.IsUgen,
		&resp.PerUserPrice,
		&resp.PerUserCurrencyId,
	)
	if err != nil {
		return nil, errors.Wrap(err, "projectRepo.GetById")
	}

	if currenciesString.Valid {
		err := json.Unmarshal([]byte(currenciesString.String), &currencies)
		if err != nil {
			return nil, err
		}
	}

	if subscriptionInfo.Valid {
		var subInfo struct {
			EndDate string `json:"end_date"`
			Type    string `json:"type"`
		}
		err := json.Unmarshal([]byte(subscriptionInfo.String), &subInfo)
		if err != nil {
			return nil, err
		}
		resp.ExpireDate = subInfo.EndDate
		resp.SubscriptionType = subInfo.Type
	}

	query := `SELECT id, name, short_name, native_name FROM language WHERE id = ANY($1)`
	row, err := c.db.Query(ctx, query, lan_ids)
	if err != nil {
		return nil, errors.Wrap(err, "projectRepo.GetById")
	}

	defer row.Close()
	for row.Next() {
		var lang pb.Language
		err = row.Scan(
			&lang.Id,
			&lang.Name,
			&lang.ShortName,
			&lang.NativeName,
		)
		if err != nil {
			return nil, errors.Wrap(err, "projectRepo.GetById")
		}
		lan = append(lan, &lang)
	}

	resp.Language = lan
	resp.Currency = &cur
	resp.Timezone = &tzone
	resp.FareId = fareId.String
	resp.Currencies = currencies

	return &resp, nil
}

// GetUgenProjectByCompanyId returns the company's single head (is_ugen) project.
// Uniqueness of the ugen project per company is guaranteed by validateUgenUniqueness,
// so this returns at most one row. The balance it carries is the one charged for
// paid template imports and paid user seats.
func (c *projectRepo) GetUgenProjectByCompanyId(ctx context.Context, companyId string) (*pb.Project, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetUgenProjectByCompanyId")
	defer dbSpan.Finish()

	var resp pb.Project
	stmt := `SELECT id, company_id, COALESCE(k8s_namespace, ''), COALESCE(title, ''),
				COALESCE(balance, 0), COALESCE(credit_limit, 0), is_ugen,
				COALESCE(per_user_price, 0), COALESCE(per_user_currency_id::text, '')
			 FROM project
			 WHERE company_id = $1 AND is_ugen = true AND deleted_at IS NULL
			 LIMIT 1`

	err := c.db.QueryRow(ctx, stmt, companyId).Scan(
		&resp.ProjectId, &resp.CompanyId, &resp.K8SNamespace, &resp.Title,
		&resp.Balance, &resp.CreditLimit, &resp.IsUgen,
		&resp.PerUserPrice, &resp.PerUserCurrencyId,
	)
	if err != nil {
		return nil, errors.Wrap(err, "projectRepo.GetUgenProjectByCompanyId")
	}

	return &resp, nil
}

func (c *projectRepo) GetList(ctx context.Context, queryParam *pb.GetProjectListRequest) (*pb.GetProjectListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetList")
	defer dbSpan.Finish()

	var (
		res    = &pb.GetProjectListResponse{}
		params = make(map[string]any)

		arr   []any
		query = `SELECT
					p.id,
					p.title,
					p.company_id,
					p.k8s_namespace,
					p.logo,
					p.fare_id,
					p.balance,
					p.credit_limit,
					p.created_at,
					p.updated_at,
					p.status,
					f.id AS fare_id, 
					f.name AS fare_name,
					(
						SELECT json_build_object('end_date', s.end_date, 'type', s.type)
						FROM subscription s
						WHERE s.project_id = p.id
						ORDER BY s.created_at DESC
						LIMIT 1
					) AS subscription_info,
					p.new_design,
					p.new_layout,
					p.new_router,
					p.is_ugen

				FROM
					"project" p
				LEFT JOIN
					"fare" f
				ON
					p.fare_id = f.id
			`
		filter      = " WHERE is_deleted=false"
		order       = " ORDER BY created_at"
		arrangement = " DESC"
		offset      = " OFFSET 0"
		limit       = " LIMIT 10"
	)

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND ((title) ILIKE ('%' || :search || '%'))"
	}

	if len(queryParam.CompanyId) > 0 {
		params["company_id"] = queryParam.CompanyId
		filter += " AND company_id = :company_id"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "project"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(&res.Count)
	if err != nil {
		return res, err
	}

	q := query + filter + order + arrangement + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			createdAt, updatedAt, fareId sql.NullString
			fId, fName, status, logo     sql.NullString
			subscriptionInfo             sql.NullString

			obj = &pb.Project{}
		)

		err = rows.Scan(
			&obj.ProjectId,
			&obj.Title,
			&obj.CompanyId,
			&obj.K8SNamespace,
			&logo,
			&fareId,
			&obj.Balance,
			&obj.CreditLimit,
			&createdAt,
			&updatedAt,
			&status,
			&fId,
			&fName,
			&subscriptionInfo,
			&obj.NewDesign,
			&obj.NewLayout,
			&obj.NewRouter,
			&obj.IsUgen,
		)

		if err != nil {
			return res, err
		}

		if subscriptionInfo.Valid {
			var subInfo struct {
				EndDate string `json:"end_date"`
				Type    string `json:"type"`
			}
			err := json.Unmarshal([]byte(subscriptionInfo.String), &subInfo)
			if err != nil {
				return res, err
			}
			obj.ExpireDate = subInfo.EndDate
			obj.SubscriptionType = subInfo.Type
		}

		obj.Logo = logo.String
		obj.FareId = fareId.String
		obj.Status = status.String

		res.Projects = append(res.Projects, obj)
	}

	return res, nil
}

func (c *projectRepo) Update(ctx context.Context, project *pb.Project) (*pb.Project, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Update")
	defer dbSpan.Finish()

	var (
		err   error
		check bool
		lan   []string

		params      = make(map[string]any)
		updateQuery = `
			UPDATE project
			SET 
			  title = :title,
			  logo = :logo,
			  new_design = :new_design,
			  new_layout = :new_layout,
			  new_router = :new_router,
			  icon_categories = :icon_categories,
			  updated_at = current_timestamp
		`
		filter = " WHERE id = :id and company_id = :company_id"
	)

	params["title"] = project.GetTitle()
	params["logo"] = project.GetLogo()
	params["id"] = project.GetProjectId()
	params["company_id"] = project.GetCompanyId()
	params["new_design"] = project.GetNewDesign()
	params["new_layout"] = project.GetNewLayout()
	params["new_router"] = project.GetNewRouter()
	params["icon_categories"] = project.GetIconCategories()

	if project.Balance != 0 || project.BalanceSet {
		params["balance"] = project.GetBalance()
		updateQuery += `, balance = :balance`
	}

	if project.Status != "" {
		params["status"] = project.GetStatus()
		updateQuery += `, status = :status`
	}

	for _, value := range project.GetLanguage() {
		if !util.IsValidUUID(value.GetId()) {
			check = true
		} else {
			lan = append(lan, value.Id)
		}
	}
	if !check {
		params["language_id"] = pq.Array(lan)
		updateQuery += `, language_id = :language_id`
	}

	if util.IsValidUUID(project.GetCurrency().GetId()) {
		params["currency_id"] = project.GetCurrency().GetId()
		updateQuery += `, currency_id = :currency_id`
	}

	if util.IsValidUUID(project.GetTimezone().GetId()) {
		params["timezone_id"] = project.GetTimezone().GetId()
		updateQuery += `, timezone_id = :timezone_id`
	}

	if project.IsUgen {
		var current bool
		err := c.db.QueryRow(ctx, `SELECT is_ugen FROM project WHERE id = $1`, project.ProjectId).Scan(&current)
		if err == nil && !current {
			if err := c.validateUgenUniqueness(ctx, project.CompanyId); err != nil {
				return nil, err
			}
		}
	}
	params["is_ugen"] = project.GetIsUgen()
	updateQuery += `, is_ugen = :is_ugen`

	query := updateQuery + filter

	cQuery, arr := helper.ReplaceQueryParams(query, params)

	_, err = c.db.Exec(ctx, cQuery, arr...)

	if err != nil {
		return nil, err
	}

	resp, err := c.GetById(ctx, project.ProjectId)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *projectRepo) Delete(ctx context.Context, id string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Delete")
	defer dbSpan.Finish()

	softDeleteQuery := `
	UPDATE "project"
	SET
		is_deleted = true,
		deleted_at = CURRENT_TIMESTAMP
	WHERE id = $1
	`
	sqlResult, err := c.db.Exec(ctx, softDeleteQuery, id)
	if err != nil {
		return err
	}

	rowsAffected := sqlResult.RowsAffected()

	if rowsAffected < 1 {
		return pgx.ErrNoRows
	}

	return nil
}

func (c *projectRepo) DeleteAfterTest(ctx context.Context, id string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.DeleteAfterTest")
	defer dbSpan.Finish()

	deleteQuery := `
	DELETE FROM "project"
	WHERE id = $1 AND is_deleted = true
	`
	sqlResult, err := c.db.Exec(ctx, deleteQuery, id)
	if err != nil {
		return err
	}

	rowsAffected := sqlResult.RowsAffected()

	if rowsAffected < 1 {
		return pgx.ErrNoRows
	}

	return nil
}

func (c *projectRepo) GetProjectsByCompanyId(ctx context.Context, req *pb.GetProjectsByCompanyIdReq) (*pb.GetProjectsByCompanyIdRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetProjectsByCompanyId")
	defer dbSpan.Finish()

	company := pb.GetProjectsByCompanyIdRes_Company{
		Projects: []*pb.GetProjectsByCompanyIdRes_Company_Project{},
	}

	query1 := `SELECT id,
       				title,
       				coalesce(logo, ''),
       				coalesce(description, ''),
       				coalesce(created_at::text, ''),
       				coalesce(updated_at::text, ''),
       				owner_id
			FROM company
			WHERE id = $1`

	err := c.db.QueryRow(ctx, query1, req.GetCompanyId()).Scan(
		&company.Id,
		&company.Name,
		&company.Logo,
		&company.Description,
		&company.CreatedAt,
		&company.UpdatedAt,
		&company.OwnerId,
	)
	if err != nil {
		return nil, err
	}

	query2 := `SELECT id,
       				company_id,
       				title,
       				coalesce(k8s_namespace, ''),
       				coalesce(created_at::text, ''),
       				coalesce(updated_at::text, '')
			FROM project
			WHERE company_id = $1`

	rows, err := c.db.Query(ctx, query2, req.GetCompanyId())
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var project pb.GetProjectsByCompanyIdRes_Company_Project
		err = rows.Scan(
			&project.Id,
			&project.CompanyId,
			&project.Name,
			&project.Domain,
			&project.CreatedAt,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		company.Projects = append(company.Projects, &project)
	}

	return &pb.GetProjectsByCompanyIdRes{
		Company: &company,
	}, nil
}

func (c *projectRepo) GetProjectResources(ctx context.Context, in *pb.GetProjectsRequest) (*pb.GetProjectRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetProjectResources")
	defer dbSpan.Finish()

	var (
		resp      = &pb.GetProjectRes{}
		createdAt sql.NullString
		updatedAt sql.NullString

		stmt = `
		SELECT
			id,
			title ,
			company_id ,
			updated_at,
			created_at
		FROM project
		WHERE k8s_namespace = $1 and deleted_at IS NULL;`
	)

	row, err := c.db.Query(ctx, stmt, in.K8SNamespace)
	for row.Next() {
		var res = &pb.GetProjectsResponse{}
		err := row.Scan(
			&res.Id,
			&res.Title,
			&res.CompanyId,
			&updatedAt,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}
		res.CreatedAt = createdAt.String
		res.UpdatedAt = updatedAt.String
		resp.Response = append(resp.Response, res)
	}
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *projectRepo) GetListLanguage(ctx context.Context, in *pb.GetListSettingReq) (*models.ListLanguage, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetListLanguage")
	defer dbSpan.Finish()

	var (
		res    models.ListLanguage
		params = make(map[string]any)
		arr    []any

		query = `SELECT
				id,
				name,
				short_name,
				native_name
			FROM
				"language"`
		filter = " "
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
	)

	if len(in.GetSearch()) > 0 {
		params["search"] = in.GetSearch()
		filter = " WHERE (((name) ILIKE ('%' || :search || '%'))" +
			" OR ((short_name) ILIKE ('%' || :search || '%'))" +
			" OR ((native_name) ILIKE ('%' || :search || '%')))"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "language"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return &res, err
	}

	q := query + filter + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return &res, err
	}
	defer rows.Close()

	for rows.Next() {
		var obj models.Language

		err = rows.Scan(
			&obj.Id,
			&obj.Name,
			&obj.ShortName,
			&obj.NativeName,
		)

		if err != nil {
			return &res, err
		}

		res.Language = append(res.Language, &obj)
	}

	return &res, nil
}

func (c *projectRepo) GetListCurrency(ctx context.Context, in *pb.GetListSettingReq) (*models.ListCurrency, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetListCurrency")
	defer dbSpan.Finish()

	var (
		res    models.ListCurrency
		params = make(map[string]any)
		arr    []any

		query = `SELECT
				id,
				symbol,
				"name",
				symbol_native,
				decimal_digits,
				rounding,
				code,
				name_plural
			FROM
				"currency"`
		filter = " "
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
	)

	if len(in.GetSearch()) > 0 {
		params["search"] = in.GetSearch()
		filter = " WHERE (((symbol) ILIKE ('%' || :search || '%'))" +
			"OR ((name) ILIKE ('%' || :search || '%'))" +
			"OR ((code) ILIKE ('%' || :search || '%')))"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "currency"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return &res, err
	}

	q := query + filter + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return &res, err
	}
	defer rows.Close()

	for rows.Next() {
		var obj models.Currency

		err = rows.Scan(
			&obj.Id,
			&obj.Symbol,
			&obj.Name,
			&obj.SymbolNative,
			&obj.DecimalDigits,
			&obj.Rounding,
			&obj.Code,
			&obj.NamePlural,
		)

		if err != nil {
			return &res, err
		}

		res.Currency = append(res.Currency, &obj)
	}

	return &res, nil
}

func (c *projectRepo) GetListTimezone(ctx context.Context, in *pb.GetListSettingReq) (*models.ListTimezone, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetListTimezone")
	defer dbSpan.Finish()

	var (
		res    models.ListTimezone
		params = make(map[string]any)
		arr    []any

		query = `SELECT
				id,
				"name",
				"text"
			FROM
				"timezone"`
		filter = " "
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
	)

	if len(in.GetSearch()) > 0 {
		params["search"] = in.GetSearch()
		filter = " WHERE (((name) ILIKE ('%' || :search || '%'))" +
			"OR ((text) ILIKE ('%' || :search || '%')))"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "timezone"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return &res, err
	}

	q := query + filter + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return &res, err
	}
	defer rows.Close()

	for rows.Next() {
		var obj models.Timezone

		err = rows.Scan(
			&obj.Id,
			&obj.Name,
			&obj.Text,
		)

		if err != nil {
			return &res, err
		}

		res.Timezone = append(res.Timezone, &obj)
	}

	return &res, nil
}

func (c *projectRepo) BindMicroFrontToProject(ctx context.Context, in *pb.ProjectLoginMicroFrontend) (*pb.ProjectLoginMicroFrontend, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.BindMicroFrontToProject")
	defer dbSpan.Finish()

	var (
		result pb.ProjectLoginMicroFrontend
		err    error
		id     = uuid.New()

		insertProject = `INSERT INTO project_login_microfront
		(
			id,
			project_id,
			environment_id,
			microfront_id,
			subdomain
		) VALUES ($1, $2, $3, $4, $5) RETURNING id, project_id, environment_id, microfront_id, subdomain`
	)

	err = c.db.QueryRow(
		ctx,
		insertProject,
		id,
		in.ProjectId,
		in.EnvironmentId,
		in.MicrofrontId,
		in.Subdomain,
	).Scan(&result.Id, &result.ProjectId, &result.EnvironmentId, &result.MicrofrontId, &result.Subdomain)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *projectRepo) GetProjectLoginMicroFront(ctx context.Context, in *pb.GetProjectLoginMicroFrontRequest) (*pb.ProjectLoginMicroFrontend, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetProjectLoginMicroFront")
	defer dbSpan.Finish()

	var (
		err    error
		filter string
		resp   = &pb.ProjectLoginMicroFrontend{}
		stmt   = `SELECT
				id,
				project_id,
				environment_id,
				microfront_id,
				subdomain
			FROM project_login_microfront
			WHERE `
	)

	if in.Id != "" {
		filter = in.Id
		stmt += "id = $1"
	} else if in.ProjectId != "" {
		filter = in.ProjectId
		stmt += "project_id = $1"
	} else if in.Subdomain != "" {
		filter = in.Subdomain
		stmt += "subdomain = $1"
	}

	err = c.db.QueryRow(ctx, stmt, filter).Scan(
		&resp.Id, &resp.ProjectId, &resp.EnvironmentId, &resp.MicrofrontId, &resp.Subdomain,
	)

	if err == pgx.ErrNoRows {
		return resp, nil
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *projectRepo) UpdateProjectLoginMicroFront(ctx context.Context, in *pb.ProjectLoginMicroFrontend) (*pb.ProjectLoginMicroFrontend, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.UpdateProjectLoginMicroFront")
	defer dbSpan.Finish()

	var (
		err    error
		params = make(map[string]any)
		query  = `
			UPDATE project_login_microfront
			SET 
			  microfront_id = :microfront_id,
			  subdomain = :subdomain
			WHERE id = :id
		`
	)

	params["microfront_id"] = in.GetMicrofrontId()
	params["subdomain"] = in.GetSubdomain()
	params["id"] = in.GetId()

	cQuery, arr := helper.ReplaceQueryParams(query, params)

	_, err = c.db.Exec(ctx, cQuery, arr...)

	if err != nil {
		return nil, err
	}

	resp, err := c.GetProjectLoginMicroFront(ctx, &pb.GetProjectLoginMicroFrontRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// project configs
func (c *projectRepo) CreateProjectConfig(ctx context.Context, in *pb.ProjectConfig) (*pb.ProjectConfig, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.CreateProjectConfig")
	defer dbSpan.Finish()

	var (
		result = &pb.ProjectConfig{}
		err    error
		id     = uuid.New()
	)

	query := `
	INSERT INTO project_configs
	(
		"id",
		"project_id",
		"CONVERT_TEMPLATE_GRPC_PORT",
		"CONVERT_TEMPLATE_SERVICE_HOST",
		"HTTP_PORT",
		"HTTP_SCHEME",
		"SERVICE_HOST",
		"ANALYTICS_GRPC_PORT",
		"ANALYTICS_SERVICE_HOST",
		"API_REF_GRPC_PORT",
		"API_REF_SERVICE_HOST",
		"CHAT_GRPC_PORT",
		"CHAT_SERVICE_HOST",
		"FUNCTION_GRPC_PORT",
		"FUNCTION_SERVICE_HOST",
		"NOTIFICATION_GRPC_PORT",
		"NOTIFICATION_SERVICE_HOST",
		"OBJECT_BUILDER_GRPC_PORT",
		"OBJECT_BUILDER_HIGH_GRPC_PORT",
		"OBJECT_BUILDER_LOW_GRPC_PORT",
		"OBJECT_BUILDER_SERVICE_HIGHT_HOST",
		"OBJECT_BUILDER_SERVICE_HOST",
		"OBJECT_BUILDER_SERVICE_LOW_HOST",
		"QUERY_GRPC_PORT",
		"QUERY_SERVICE_HOST",
		"SCENARIO_GRPC_PORT",
		"SCENARIO_HTTP_PORT",
		"SCENARIO_SERVICE_HOST",
		"SMS_GRPC_PORT",
		"SMS_SERVICE_HOST",
		"TEMPLATE_GRPC_PORT",
		"TEMPLATE_SERVICE_HOST",
		"VERSIONING_GRPC_PORT",
		"VERSIONING_SERVICE_HOST",
		"MONGO_HOST",
		"MONGO_PORT",
		"COMPANY_SERVICE_MONGO_USER",
		"COMPANY_SERVICE_MONGO_PASSWORD",
		"others",
		"REDIS_HOST",
		"REDIS_PORT",
		"REDIS_DATABASE",
		"REDIS_PASSWORD"
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43)
	RETURNING 
		"id",
		"project_id",
		"CONVERT_TEMPLATE_GRPC_PORT",
		"CONVERT_TEMPLATE_SERVICE_HOST",
		"HTTP_PORT",
		"HTTP_SCHEME",
		"SERVICE_HOST",
		"ANALYTICS_GRPC_PORT",
		"ANALYTICS_SERVICE_HOST",
		"API_REF_GRPC_PORT",
		"API_REF_SERVICE_HOST",
		"CHAT_GRPC_PORT",
		"CHAT_SERVICE_HOST",
		"FUNCTION_GRPC_PORT",
		"FUNCTION_SERVICE_HOST",
		"NOTIFICATION_GRPC_PORT",
		"NOTIFICATION_SERVICE_HOST",
		"OBJECT_BUILDER_GRPC_PORT",
		"OBJECT_BUILDER_HIGH_GRPC_PORT",
		"OBJECT_BUILDER_LOW_GRPC_PORT",
		"OBJECT_BUILDER_SERVICE_HIGHT_HOST",
		"OBJECT_BUILDER_SERVICE_HOST",
		"OBJECT_BUILDER_SERVICE_LOW_HOST",
		"QUERY_GRPC_PORT",
		"QUERY_SERVICE_HOST",
		"SCENARIO_GRPC_PORT",
		"SCENARIO_HTTP_PORT",
		"SCENARIO_SERVICE_HOST",
		"SMS_GRPC_PORT",
		"SMS_SERVICE_HOST",
		"TEMPLATE_GRPC_PORT",
		"TEMPLATE_SERVICE_HOST",
		"VERSIONING_GRPC_PORT",
		"VERSIONING_SERVICE_HOST",
		"MONGO_HOST",
		"MONGO_PORT",
		"COMPANY_SERVICE_MONGO_USER",
		"COMPANY_SERVICE_MONGO_PASSWORD",
		"others",
		"REDIS_HOST",
		"REDIS_PORT",
		"REDIS_DATABASE",
		"REDIS_PASSWORD"
	`

	err = c.db.QueryRow(
		ctx,
		query,
		id,
		in.ProjectId,
		in.CONVERT_TEMPLATE_GRPC_PORT,
		in.CONVERT_TEMPLATE_SERVICE_HOST,
		in.HTTP_PORT,
		in.HTTP_SCHEME,
		in.SERVICE_HOST,
		in.ANALYTICS_GRPC_PORT,
		in.ANALYTICS_SERVICE_HOST,
		in.API_REF_GRPC_PORT,
		in.API_REF_SERVICE_HOST,
		in.CHAT_GRPC_PORT,
		in.CHAT_SERVICE_HOST,
		in.FUNCTION_GRPC_PORT,
		in.FUNCTION_SERVICE_HOST,
		in.NOTIFICATION_GRPC_PORT,
		in.NOTIFICATION_SERVICE_HOST,
		in.OBJECT_BUILDER_GRPC_PORT,
		in.OBJECT_BUILDER_HIGH_GRPC_PORT,
		in.OBJECT_BUILDER_LOW_GRPC_PORT,
		in.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
		in.OBJECT_BUILDER_SERVICE_HOST,
		in.OBJECT_BUILDER_SERVICE_LOW_HOST,
		in.QUERY_GRPC_PORT,
		in.QUERY_SERVICE_HOST,
		in.SCENARIO_GRPC_PORT,
		in.SCENARIO_HTTP_PORT,
		in.SCENARIO_SERVICE_HOST,
		in.SMS_GRPC_PORT,
		in.SMS_SERVICE_HOST,
		in.TEMPLATE_GRPC_PORT,
		in.TEMPLATE_SERVICE_HOST,
		in.VERSIONING_GRPC_PORT,
		in.VERSIONING_SERVICE_HOST,
		in.MONGO_HOST,
		in.MONGO_PORT,
		in.COMPANY_SERVICE_MONGO_USER,
		in.COMPANY_SERVICE_MONGO_PASSWORD,
		in.Others,
		in.REDIS_HOST,
		in.REDIS_PORT,
		in.REDIS_DATABASE,
		in.REDIS_PASSWORD,
	).Scan(
		&result.Id,
		&result.ProjectId,
		&result.CONVERT_TEMPLATE_GRPC_PORT,
		&result.CONVERT_TEMPLATE_SERVICE_HOST,
		&result.HTTP_PORT,
		&result.HTTP_SCHEME,
		&result.SERVICE_HOST,
		&result.ANALYTICS_GRPC_PORT,
		&result.ANALYTICS_SERVICE_HOST,
		&result.API_REF_GRPC_PORT,
		&result.API_REF_SERVICE_HOST,
		&result.CHAT_GRPC_PORT,
		&result.CHAT_SERVICE_HOST,
		&result.FUNCTION_GRPC_PORT,
		&result.FUNCTION_SERVICE_HOST,
		&result.NOTIFICATION_GRPC_PORT,
		&result.NOTIFICATION_SERVICE_HOST,
		&result.OBJECT_BUILDER_GRPC_PORT,
		&result.OBJECT_BUILDER_HIGH_GRPC_PORT,
		&result.OBJECT_BUILDER_LOW_GRPC_PORT,
		&result.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
		&result.OBJECT_BUILDER_SERVICE_HOST,
		&result.OBJECT_BUILDER_SERVICE_LOW_HOST,
		&result.QUERY_GRPC_PORT,
		&result.QUERY_SERVICE_HOST,
		&result.SCENARIO_GRPC_PORT,
		&result.SCENARIO_HTTP_PORT,
		&result.SCENARIO_SERVICE_HOST,
		&result.SMS_GRPC_PORT,
		&result.SMS_SERVICE_HOST,
		&result.TEMPLATE_GRPC_PORT,
		&result.TEMPLATE_SERVICE_HOST,
		&result.VERSIONING_GRPC_PORT,
		&result.VERSIONING_SERVICE_HOST,
		&result.MONGO_HOST,
		&result.MONGO_PORT,
		&result.COMPANY_SERVICE_MONGO_USER,
		&result.COMPANY_SERVICE_MONGO_PASSWORD,
		&result.Others,
		&result.REDIS_HOST,
		&result.REDIS_PORT,
		&result.REDIS_DATABASE,
		&result.REDIS_PASSWORD,
	)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *projectRepo) GetListProjectConfig(ctx context.Context, in *emptypb.Empty) (*pb.ListPorjectConfig, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetListProjectConfig")
	defer dbSpan.Finish()

	var result = &pb.ListPorjectConfig{}

	query :=
		`SELECT 
			"id",
			"project_id",
			"CONVERT_TEMPLATE_GRPC_PORT",
			"CONVERT_TEMPLATE_SERVICE_HOST",
			"HTTP_PORT",
			"HTTP_SCHEME",
			"SERVICE_HOST",
			"ANALYTICS_GRPC_PORT",
			"ANALYTICS_SERVICE_HOST",
			"API_REF_GRPC_PORT",
			"API_REF_SERVICE_HOST",
			"CHAT_GRPC_PORT",
			"CHAT_SERVICE_HOST",
			"FUNCTION_GRPC_PORT",
			"FUNCTION_SERVICE_HOST",
			"NOTIFICATION_GRPC_PORT",
			"NOTIFICATION_SERVICE_HOST",
			"OBJECT_BUILDER_GRPC_PORT",
			"OBJECT_BUILDER_HIGH_GRPC_PORT",
			"OBJECT_BUILDER_LOW_GRPC_PORT",
			"OBJECT_BUILDER_SERVICE_HIGHT_HOST",
			"OBJECT_BUILDER_SERVICE_HOST",
			"OBJECT_BUILDER_SERVICE_LOW_HOST",
			"QUERY_GRPC_PORT",
			"QUERY_SERVICE_HOST",
			"SCENARIO_GRPC_PORT",
			"SCENARIO_HTTP_PORT",
			"SCENARIO_SERVICE_HOST",
			"SMS_GRPC_PORT",
			"SMS_SERVICE_HOST",
			"TEMPLATE_GRPC_PORT",
			"TEMPLATE_SERVICE_HOST",
			"VERSIONING_GRPC_PORT",
			"VERSIONING_SERVICE_HOST",
			"MONGO_HOST",
     		"MONGO_PORT",
     		"COMPANY_SERVICE_MONGO_USER",
     		"COMPANY_SERVICE_MONGO_PASSWORD",
			"others",
			"REDIS_HOST",
			"REDIS_PORT",
			"REDIS_DATABASE",
			"REDIS_PASSWORD"
	FROM project_configs`

	row, err := c.db.Query(ctx, query)

	for row.Next() {
		var res = &pb.ProjectConfig{}
		err := row.Scan(
			&res.Id,
			&res.ProjectId,
			&res.CONVERT_TEMPLATE_GRPC_PORT,
			&res.CONVERT_TEMPLATE_SERVICE_HOST,
			&res.HTTP_PORT,
			&res.HTTP_SCHEME,
			&res.SERVICE_HOST,
			&res.ANALYTICS_GRPC_PORT,
			&res.ANALYTICS_SERVICE_HOST,
			&res.API_REF_GRPC_PORT,
			&res.API_REF_SERVICE_HOST,
			&res.CHAT_GRPC_PORT,
			&res.CHAT_SERVICE_HOST,
			&res.FUNCTION_GRPC_PORT,
			&res.FUNCTION_SERVICE_HOST,
			&res.NOTIFICATION_GRPC_PORT,
			&res.NOTIFICATION_SERVICE_HOST,
			&res.OBJECT_BUILDER_GRPC_PORT,
			&res.OBJECT_BUILDER_HIGH_GRPC_PORT,
			&res.OBJECT_BUILDER_LOW_GRPC_PORT,
			&res.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
			&res.OBJECT_BUILDER_SERVICE_HOST,
			&res.OBJECT_BUILDER_SERVICE_LOW_HOST,
			&res.QUERY_GRPC_PORT,
			&res.QUERY_SERVICE_HOST,
			&res.SCENARIO_GRPC_PORT,
			&res.SCENARIO_HTTP_PORT,
			&res.SCENARIO_SERVICE_HOST,
			&res.SMS_GRPC_PORT,
			&res.SMS_SERVICE_HOST,
			&res.TEMPLATE_GRPC_PORT,
			&res.TEMPLATE_SERVICE_HOST,
			&res.VERSIONING_GRPC_PORT,
			&res.VERSIONING_SERVICE_HOST,
			&res.MONGO_HOST,
			&res.MONGO_PORT,
			&res.COMPANY_SERVICE_MONGO_USER,
			&res.COMPANY_SERVICE_MONGO_PASSWORD,
			&res.Others,
			&res.REDIS_HOST,
			&res.REDIS_PORT,
			&res.REDIS_DATABASE,
			&res.REDIS_PASSWORD,
		)
		if err != nil {
			return nil, err
		}
		result.Configs = append(result.Configs, res)
	}

	if err == pgx.ErrNoRows {
		return result, nil
	} else if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *projectRepo) GetProjectConfigById(ctx context.Context, in *pb.GetPorjectConfigByProjectIdRequest) (*pb.ProjectConfig, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetProjectConfigById")
	defer dbSpan.Finish()

	var (
		res = &pb.ProjectConfig{}
		err error
	)

	query :=
		`SELECT
			id,
			project_id,
			"CONVERT_TEMPLATE_GRPC_PORT",
			"CONVERT_TEMPLATE_SERVICE_HOST",
			"HTTP_PORT",
			"HTTP_SCHEME",
			"SERVICE_HOST",
			"ANALYTICS_GRPC_PORT",
			"ANALYTICS_SERVICE_HOST",
			"API_REF_GRPC_PORT",
			"API_REF_SERVICE_HOST",
			"CHAT_GRPC_PORT",
			"CHAT_SERVICE_HOST",
			"FUNCTION_GRPC_PORT",
			"FUNCTION_SERVICE_HOST",
			"NOTIFICATION_GRPC_PORT",
			"NOTIFICATION_SERVICE_HOST",
			"OBJECT_BUILDER_GRPC_PORT",
			"OBJECT_BUILDER_HIGH_GRPC_PORT",
			"OBJECT_BUILDER_LOW_GRPC_PORT",
			"OBJECT_BUILDER_SERVICE_HIGHT_HOST",
			"OBJECT_BUILDER_SERVICE_HOST",
			"OBJECT_BUILDER_SERVICE_LOW_HOST",
			"QUERY_GRPC_PORT",
			"QUERY_SERVICE_HOST",
			"SCENARIO_GRPC_PORT",
			"SCENARIO_HTTP_PORT",
			"SCENARIO_SERVICE_HOST",
			"SMS_GRPC_PORT",
			"SMS_SERVICE_HOST",
			"TEMPLATE_GRPC_PORT",
			"TEMPLATE_SERVICE_HOST",
			"VERSIONING_GRPC_PORT",
			"VERSIONING_SERVICE_HOST",
			"MONGO_HOST",
			"MONGO_PORT",
			"COMPANY_SERVICE_MONGO_USER",
			"COMPANY_SERVICE_MONGO_PASSWORD",
			others,
			"REDIS_HOST",
			"REDIS_PORT",
			"REDIS_DATABASE",
			"REDIS_PASSWORD"
	FROM project_configs
	WHERE project_id = $1`

	err = c.db.QueryRow(ctx, query, in.ProjectId).Scan(
		&res.Id,
		&res.ProjectId,
		&res.CONVERT_TEMPLATE_GRPC_PORT,
		&res.CONVERT_TEMPLATE_SERVICE_HOST,
		&res.HTTP_PORT,
		&res.HTTP_SCHEME,
		&res.SERVICE_HOST,
		&res.ANALYTICS_GRPC_PORT,
		&res.ANALYTICS_SERVICE_HOST,
		&res.API_REF_GRPC_PORT,
		&res.API_REF_SERVICE_HOST,
		&res.CHAT_GRPC_PORT,
		&res.CHAT_SERVICE_HOST,
		&res.FUNCTION_GRPC_PORT,
		&res.FUNCTION_SERVICE_HOST,
		&res.NOTIFICATION_GRPC_PORT,
		&res.NOTIFICATION_SERVICE_HOST,
		&res.OBJECT_BUILDER_GRPC_PORT,
		&res.OBJECT_BUILDER_HIGH_GRPC_PORT,
		&res.OBJECT_BUILDER_LOW_GRPC_PORT,
		&res.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
		&res.OBJECT_BUILDER_SERVICE_HOST,
		&res.OBJECT_BUILDER_SERVICE_LOW_HOST,
		&res.QUERY_GRPC_PORT,
		&res.QUERY_SERVICE_HOST,
		&res.SCENARIO_GRPC_PORT,
		&res.SCENARIO_HTTP_PORT,
		&res.SCENARIO_SERVICE_HOST,
		&res.SMS_GRPC_PORT,
		&res.SMS_SERVICE_HOST,
		&res.TEMPLATE_GRPC_PORT,
		&res.TEMPLATE_SERVICE_HOST,
		&res.VERSIONING_GRPC_PORT,
		&res.VERSIONING_SERVICE_HOST,
		&res.MONGO_HOST,
		&res.MONGO_PORT,
		&res.COMPANY_SERVICE_MONGO_USER,
		&res.COMPANY_SERVICE_MONGO_PASSWORD,
		&res.Others,
		&res.REDIS_HOST,
		&res.REDIS_PORT,
		&res.REDIS_DATABASE,
		&res.REDIS_PASSWORD,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *projectRepo) UpdateProjectConfig(ctx context.Context, in *pb.ProjectConfig) (*pb.ProjectConfig, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.UpdateProjectConfig")
	defer dbSpan.Finish()

	var (
		err error
		res = &pb.ProjectConfig{}
	)

	query := `
		UPDATE project_configs
		SET 
			"CONVERT_TEMPLATE_GRPC_PORT" = $3,
			"CONVERT_TEMPLATE_SERVICE_HOST" = $4,
			"HTTP_PORT" = $5,
			"HTTP_SCHEME" = $6,
			"SERVICE_HOST" = $7,
			"ANALYTICS_GRPC_PORT" = $8,
			"ANALYTICS_SERVICE_HOST" = $9,
			"API_REF_GRPC_PORT" = $10,
			"API_REF_SERVICE_HOST" = $11,
			"CHAT_GRPC_PORT" = $12,
			"CHAT_SERVICE_HOST" = $13,
			"FUNCTION_GRPC_PORT" = $14,
			"FUNCTION_SERVICE_HOST" = $15,
			"NOTIFICATION_GRPC_PORT" = $16,
			"NOTIFICATION_SERVICE_HOST" = $17,
			"OBJECT_BUILDER_GRPC_PORT" = $18,
			"OBJECT_BUILDER_HIGH_GRPC_PORT" = $19,
			"OBJECT_BUILDER_LOW_GRPC_PORT" = $20,
			"OBJECT_BUILDER_SERVICE_HIGHT_HOST" = $21,
			"OBJECT_BUILDER_SERVICE_HOST" = $22,
			"OBJECT_BUILDER_SERVICE_LOW_HOST" = $23,
			"QUERY_GRPC_PORT" = $24,
			"QUERY_SERVICE_HOST" = $25,
			"SCENARIO_GRPC_PORT" = $26,
			"SCENARIO_HTTP_PORT" = $27,
			"SCENARIO_SERVICE_HOST" = $28,
			"SMS_GRPC_PORT" = $29,
			"SMS_SERVICE_HOST" = $30,
			"TEMPLATE_GRPC_PORT" = $31,
			"TEMPLATE_SERVICE_HOST" = $32,
			"VERSIONING_GRPC_PORT" = $33,
			"VERSIONING_SERVICE_HOST" = $34,
			"MONGO_HOST" = $35,
			"MONGO_PORT" = $36,
			"COMPANY_SERVICE_MONGO_USER" = $37,
			"COMPANY_SERVICE_MONGO_PASSWORD" = $38,
			others = $39,
			"REDIS_HOST" = $40,
			"REDIS_PORT" = $41,
			"REDIS_DATABASE" = $42,
			"REDIS_PASSWORD" = $43
		WHERE id = $1 and project_id = $2
		RETURNING 
			"id",
			"project_id",
			"CONVERT_TEMPLATE_GRPC_PORT",
			"CONVERT_TEMPLATE_SERVICE_HOST",
			"HTTP_PORT",
			"HTTP_SCHEME",
			"SERVICE_HOST",
			"ANALYTICS_GRPC_PORT",
			"ANALYTICS_SERVICE_HOST",
			"API_REF_GRPC_PORT",
			"API_REF_SERVICE_HOST",
			"CHAT_GRPC_PORT",
			"CHAT_SERVICE_HOST",
			"FUNCTION_GRPC_PORT",
			"FUNCTION_SERVICE_HOST",
			"NOTIFICATION_GRPC_PORT",
			"NOTIFICATION_SERVICE_HOST",
			"OBJECT_BUILDER_GRPC_PORT",
			"OBJECT_BUILDER_HIGH_GRPC_PORT",
			"OBJECT_BUILDER_LOW_GRPC_PORT",
			"OBJECT_BUILDER_SERVICE_HIGHT_HOST",
			"OBJECT_BUILDER_SERVICE_HOST",
			"OBJECT_BUILDER_SERVICE_LOW_HOST",
			"QUERY_GRPC_PORT",
			"QUERY_SERVICE_HOST",
			"SCENARIO_GRPC_PORT",
			"SCENARIO_HTTP_PORT",
			"SCENARIO_SERVICE_HOST",
			"SMS_GRPC_PORT",
			"SMS_SERVICE_HOST",
			"TEMPLATE_GRPC_PORT",
			"TEMPLATE_SERVICE_HOST",
			"VERSIONING_GRPC_PORT",
			"VERSIONING_SERVICE_HOST",
			"MONGO_HOST",
			"MONGO_PORT",
			"COMPANY_SERVICE_MONGO_USER",
			"COMPANY_SERVICE_MONGO_PASSWORD",
			"others",
			"REDIS_HOST",
			"REDIS_PORT",
			"REDIS_DATABASE",
			"REDIS_PASSWORD"
	`

	err = c.db.QueryRow(
		ctx,
		query,
		in.Id,
		in.ProjectId,
		in.CONVERT_TEMPLATE_GRPC_PORT,
		in.CONVERT_TEMPLATE_SERVICE_HOST,
		in.HTTP_PORT,
		in.HTTP_SCHEME,
		in.SERVICE_HOST,
		in.ANALYTICS_GRPC_PORT,
		in.ANALYTICS_SERVICE_HOST,
		in.API_REF_GRPC_PORT,
		in.API_REF_SERVICE_HOST,
		in.CHAT_GRPC_PORT,
		in.CHAT_SERVICE_HOST,
		in.FUNCTION_GRPC_PORT,
		in.FUNCTION_SERVICE_HOST,
		in.NOTIFICATION_GRPC_PORT,
		in.NOTIFICATION_SERVICE_HOST,
		in.OBJECT_BUILDER_GRPC_PORT,
		in.OBJECT_BUILDER_HIGH_GRPC_PORT,
		in.OBJECT_BUILDER_LOW_GRPC_PORT,
		in.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
		in.OBJECT_BUILDER_SERVICE_HOST,
		in.OBJECT_BUILDER_SERVICE_LOW_HOST,
		in.QUERY_GRPC_PORT,
		in.QUERY_SERVICE_HOST,
		in.SCENARIO_GRPC_PORT,
		in.SCENARIO_HTTP_PORT,
		in.SCENARIO_SERVICE_HOST,
		in.SMS_GRPC_PORT,
		in.SMS_SERVICE_HOST,
		in.TEMPLATE_GRPC_PORT,
		in.TEMPLATE_SERVICE_HOST,
		in.VERSIONING_GRPC_PORT,
		in.VERSIONING_SERVICE_HOST,
		in.MONGO_HOST,
		in.MONGO_PORT,
		in.COMPANY_SERVICE_MONGO_USER,
		in.COMPANY_SERVICE_MONGO_PASSWORD,
		in.Others,
		in.REDIS_HOST,
		in.REDIS_PORT,
		in.REDIS_DATABASE,
		in.REDIS_PASSWORD,
	).Scan(
		&res.Id,
		&res.ProjectId,
		&res.CONVERT_TEMPLATE_GRPC_PORT,
		&res.CONVERT_TEMPLATE_SERVICE_HOST,
		&res.HTTP_PORT,
		&res.HTTP_SCHEME,
		&res.SERVICE_HOST,
		&res.ANALYTICS_GRPC_PORT,
		&res.ANALYTICS_SERVICE_HOST,
		&res.API_REF_GRPC_PORT,
		&res.API_REF_SERVICE_HOST,
		&res.CHAT_GRPC_PORT,
		&res.CHAT_SERVICE_HOST,
		&res.FUNCTION_GRPC_PORT,
		&res.FUNCTION_SERVICE_HOST,
		&res.NOTIFICATION_GRPC_PORT,
		&res.NOTIFICATION_SERVICE_HOST,
		&res.OBJECT_BUILDER_GRPC_PORT,
		&res.OBJECT_BUILDER_HIGH_GRPC_PORT,
		&res.OBJECT_BUILDER_LOW_GRPC_PORT,
		&res.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
		&res.OBJECT_BUILDER_SERVICE_HOST,
		&res.OBJECT_BUILDER_SERVICE_LOW_HOST,
		&res.QUERY_GRPC_PORT,
		&res.QUERY_SERVICE_HOST,
		&res.SCENARIO_GRPC_PORT,
		&res.SCENARIO_HTTP_PORT,
		&res.SCENARIO_SERVICE_HOST,
		&res.SMS_GRPC_PORT,
		&res.SMS_SERVICE_HOST,
		&res.TEMPLATE_GRPC_PORT,
		&res.TEMPLATE_SERVICE_HOST,
		&res.VERSIONING_GRPC_PORT,
		&res.VERSIONING_SERVICE_HOST,
		&res.MONGO_HOST,
		&res.MONGO_PORT,
		&res.COMPANY_SERVICE_MONGO_USER,
		&res.COMPANY_SERVICE_MONGO_PASSWORD,
		&res.Others,
		&res.REDIS_HOST,
		&res.REDIS_PORT,
		&res.REDIS_DATABASE,
		&res.REDIS_PASSWORD,
	)

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (p *projectRepo) ListProjectRPS(ctx context.Context, in *pb.GetProjectListRequest) (*pb.ListProjectsRPSResponse, error) {
	projects := make(map[string]int32, 0)

	query := `
		SELECT 
			p.id AS project_id,
			NULLIF(fip.value, '')::INTEGER AS request_per_second
		FROM project p
		JOIN fare_item_price fip ON p.fare_id = fip.fare_id
		WHERE fip.item_type = 'request_per_second';
	`

	rows, err := p.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			projectId string
			requests  int32
		)
		err := rows.Scan(&projectId, &requests)
		if err != nil {
			return nil, err
		}
		projects[projectId] = requests
	}

	return &pb.ListProjectsRPSResponse{
		Projects: projects,
	}, nil
}

func (p *projectRepo) AttachCustomer(ctx context.Context, in *pb.AttachCustomerRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.AttachCustomer")
	defer dbSpan.Finish()

	updateQuery := `UPDATE project SET customer_id = $1 WHERE id = $2`
	_, err := p.db.Exec(ctx, updateQuery, in.CustomerId, in.ProjectId)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "attach customer to project")
	}

	return &emptypb.Empty{}, nil
}

func (p *projectRepo) UpdateProjectBalance(ctx context.Context, projectId string, newBalance float64) error {

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.UpdateProjectBalance")
	defer dbSpan.Finish()

	var query = "UPDATE project SET balance = $2 WHERE id = $1"

	_, err := p.db.Exec(ctx, query, projectId, newBalance)
	if err != nil {
		return p.db.HandleDatabaseError(err, "update project balance")
	}

	return nil
}

func (p *projectRepo) validateUgenUniqueness(ctx context.Context, companyId string) error {
	var (
		count int
		query = `SELECT count(1) FROM project WHERE company_id = $1 AND is_ugen = true AND deleted_at IS NULL`
	)

	err := p.db.QueryRow(ctx, query, companyId).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return errors.New("company already has a ugen project")
	}

	return nil
}

func (p *projectRepo) GetProjectUgenStatus(ctx context.Context, req *pb.GetProjectUgenStatusRequest) (*pb.GetProjectUgenStatusResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetProjectUgenStatus")
	defer dbSpan.Finish()

	var (
		isUgen sql.NullBool
		count  int

		query = `SELECT
				(SELECT is_ugen FROM project WHERE id = $1 AND deleted_at IS NULL),
				(SELECT count(1) FROM project WHERE company_id = $2 AND deleted_at IS NULL)
		`
	)

	err := p.db.QueryRow(ctx, query, req.ProjectId, req.CompanyId).Scan(&isUgen, &count)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, p.db.HandleDatabaseError(err, "GetProjectUgenStatus")
	}

	return &pb.GetProjectUgenStatusResponse{
		IsUgen:               isUgen.Bool,
		CompanyProjectsCount: int32(count),
	}, nil
}

func (p *projectRepo) UpdateProjectUgenAccess(ctx context.Context, req *pb.UpdateProjectUgenAccessRequest) (*pb.UpdateProjectUgenAccessResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.UpdateProjectUgenAccess")
	defer dbSpan.Finish()

	tx, err := p.db.Begin(ctx)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "UpdateProjectUgenAccess begin tx")
	}
	defer tx.Rollback(ctx)

	var (
		count int
		query = `SELECT count(1) FROM project WHERE company_id = $1 AND is_ugen = true AND deleted_at IS NULL AND id != $2`
	)

	err = tx.QueryRow(ctx, query, req.CompanyId, req.ProjectId).Scan(&count)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "UpdateProjectUgenAccess validate")
	}
	if count > 0 {
		return nil, errors.New("company already has a ugen project")
	}

	query = `UPDATE project SET is_ugen = true WHERE id = $1 AND company_id = $2 AND deleted_at IS NULL`
	_, err = tx.Exec(ctx, query, req.ProjectId, req.CompanyId)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "UpdateProjectUgenAccess set true")
	}

	query = `UPDATE project SET is_ugen = false WHERE id != $1 AND company_id = $2 AND deleted_at IS NULL`
	_, err = tx.Exec(ctx, query, req.ProjectId, req.CompanyId)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "UpdateProjectUgenAccess set false")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, p.db.HandleDatabaseError(err, "UpdateProjectUgenAccess commit")
	}

	project, err := p.GetById(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}

	return &pb.UpdateProjectUgenAccessResponse{
		Project: project,
	}, nil
}

func (p *projectRepo) AutoAssignUgenIfSingle(ctx context.Context, req *pb.AutoAssignUgenIfSingleRequest) (*pb.AutoAssignUgenIfSingleResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.AutoAssignUgenIfSingle")
	defer dbSpan.Finish()

	var (
		count int
		query = `SELECT count(1) FROM project WHERE company_id = $1 AND deleted_at IS NULL`
	)
	err := p.db.QueryRow(ctx, query, req.CompanyId).Scan(&count)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "AutoAssignUgenIfSingle count")
	}

	if count == 1 {
		query = `UPDATE project SET is_ugen = true WHERE company_id = $1 AND deleted_at IS NULL`
		_, err = p.db.Exec(ctx, query, req.CompanyId)
		if err != nil {
			return nil, p.db.HandleDatabaseError(err, "AutoAssignUgenIfSingle update")
		}
		return &pb.AutoAssignUgenIfSingleResponse{Assigned: true}, nil
	}

	return &pb.AutoAssignUgenIfSingleResponse{Assigned: false}, nil
}

func (p *projectRepo) ListUgenProjects(ctx context.Context, req *pb.ListUgenProjectsRequest) (*pb.ListUgenProjectsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.ListUgenProjects")
	defer dbSpan.Finish()

	limit := req.GetLimit()
	if limit <= 0 {
		limit = 10
	}
	offset := req.GetOffset()
	if offset < 0 {
		offset = 0
	}

	var query = `
		SELECT
			p.id,
			p.title,
			p.company_id,
			p.k8s_namespace,
			p.logo,
			p.fare_id,
			p.balance,
			p.credit_limit,
			p.status,
			p.created_at,
			p.updated_at,
			COALESCE(e.id::text, '')        AS environment_id,
			cp.cnt                          AS company_projects_count,
			au.last_activity                AS last_activity_date,
			COUNT(*) OVER()                 AS total_count
		FROM project p
		LEFT JOIN LATERAL (
			SELECT id
			FROM environment
			WHERE project_id = p.id
			ORDER BY (name = 'Production') DESC, created_at ASC
			LIMIT 1
		) e ON TRUE
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::int AS cnt
			FROM project
			WHERE company_id = p.company_id
			  AND deleted_at IS NULL
		) cp ON TRUE
		LEFT JOIN LATERAL (
			SELECT MAX(created_at) AS last_activity
			FROM ai_token_usage
			WHERE company_id = p.company_id
		) au ON TRUE
		WHERE p.is_ugen = TRUE
		  AND p.deleted_at IS NULL
		  AND p.company_id IS NOT NULL
		  AND ($1::text = '' OR p.title ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := p.db.Query(ctx, query, req.GetSearch(), limit, offset)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "ListUgenProjects query")
	}
	defer rows.Close()

	res := &pb.ListUgenProjectsResponse{
		Projects: []*pb.UgenProjectItem{},
	}

	var total int32
	for rows.Next() {
		var (
			item                            = &pb.UgenProjectItem{}
			logo, fareID, statusVal         sql.NullString
			createdAt, updatedAt            sql.NullTime
			lastActivity                    sql.NullTime
			companyProjectsCount, totalRows int32
		)

		if err := rows.Scan(
			&item.ProjectId,
			&item.Title,
			&item.CompanyId,
			&item.K8SNamespace,
			&logo,
			&fareID,
			&item.Balance,
			&item.CreditLimit,
			&statusVal,
			&createdAt,
			&updatedAt,
			&item.EnvironmentId,
			&companyProjectsCount,
			&lastActivity,
			&totalRows,
		); err != nil {
			return nil, p.db.HandleDatabaseError(err, "ListUgenProjects scan")
		}

		item.Logo = logo.String
		item.FareId = fareID.String
		item.Status = statusVal.String
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		if updatedAt.Valid {
			item.UpdatedAt = updatedAt.Time.Format(time.RFC3339)
		}
		if lastActivity.Valid {
			item.LastActivityDate = lastActivity.Time.Format(time.RFC3339)
		}
		item.CompanyProjectsCount = companyProjectsCount
		total = totalRows

		res.Projects = append(res.Projects, item)
	}
	if err := rows.Err(); err != nil {
		return nil, p.db.HandleDatabaseError(err, "ListUgenProjects rows")
	}

	res.Count = total
	return res, nil
}

func (p *projectRepo) ListAllUgenProjects(ctx context.Context, search string) ([]*pb.UgenProjectItem, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.ListAllUgenProjects")
	defer dbSpan.Finish()

	var query = `
		SELECT
			p.id,
			p.title,
			p.company_id,
			p.k8s_namespace,
			p.logo,
			p.fare_id,
			p.balance,
			p.credit_limit,
			p.status,
			p.created_at,
			p.updated_at,
			COALESCE(e.id::text, '')        AS environment_id,
			cp.cnt                          AS company_projects_count,
			au.last_activity                AS last_activity_date
		FROM project p
		LEFT JOIN LATERAL (
			SELECT id
			FROM environment
			WHERE project_id = p.id
			ORDER BY (name = 'Production') DESC, created_at ASC
			LIMIT 1
		) e ON TRUE
		LEFT JOIN LATERAL (
			SELECT COUNT(*)::int AS cnt
			FROM project
			WHERE company_id = p.company_id
			  AND deleted_at IS NULL
		) cp ON TRUE
		LEFT JOIN LATERAL (
			SELECT MAX(created_at) AS last_activity
			FROM ai_token_usage
			WHERE company_id = p.company_id
		) au ON TRUE
		WHERE p.is_ugen = TRUE
		  AND p.deleted_at IS NULL
		  AND p.company_id IS NOT NULL
		  AND ($1::text = '' OR p.title ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC
	`

	rows, err := p.db.Query(ctx, query, search)
	if err != nil {
		return nil, p.db.HandleDatabaseError(err, "ListAllUgenProjects query")
	}
	defer rows.Close()

	projects := []*pb.UgenProjectItem{}
	for rows.Next() {
		var (
			item                    = &pb.UgenProjectItem{}
			logo, fareID, statusVal sql.NullString
			createdAt, updatedAt    sql.NullTime
			lastActivity            sql.NullTime
			companyProjectsCount    int32
		)

		if err := rows.Scan(
			&item.ProjectId,
			&item.Title,
			&item.CompanyId,
			&item.K8SNamespace,
			&logo,
			&fareID,
			&item.Balance,
			&item.CreditLimit,
			&statusVal,
			&createdAt,
			&updatedAt,
			&item.EnvironmentId,
			&companyProjectsCount,
			&lastActivity,
		); err != nil {
			return nil, p.db.HandleDatabaseError(err, "ListAllUgenProjects scan")
		}

		item.Logo = logo.String
		item.FareId = fareID.String
		item.Status = statusVal.String
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		if updatedAt.Valid {
			item.UpdatedAt = updatedAt.Time.Format(time.RFC3339)
		}
		if lastActivity.Valid {
			item.LastActivityDate = lastActivity.Time.Format(time.RFC3339)
		}
		item.CompanyProjectsCount = companyProjectsCount

		projects = append(projects, item)
	}
	if err := rows.Err(); err != nil {
		return nil, p.db.HandleDatabaseError(err, "ListAllUgenProjects rows")
	}

	return projects, nil
}
