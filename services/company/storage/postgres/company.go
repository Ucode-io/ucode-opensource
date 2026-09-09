package postgres

import (
	"context"
	"database/sql"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"
)

type companyRepo struct {
	db *Pool
}

func NewCompanyRepo(db *Pool) repo.CompanyStorageI {
	return &companyRepo{
		db: db,
	}
}

func (c *companyRepo) Create(ctx context.Context, company *pb.CreateCompanyRequest) (*pb.CreateCompanyResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.Create")
	defer dbSpan.Finish()

	var (
		err           error
		result        string
		companyId     = uuid.New()
		insertCompany = `INSERT INTO company(
			id,
			title,
			logo,
			description,
			owner_id
		) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	)

	err = c.db.QueryRow(
		ctx,
		insertCompany,
		companyId.String(),
		company.Title,
		company.Logo,
		company.Description,
		company.OwnerId,
	).Scan(&result)

	if err != nil {
		return nil, err
	}

	return &pb.CreateCompanyResponse{
		Id: result,
	}, nil
}

func (c *companyRepo) GetById(ctx context.Context, id string) (*pb.GetCompanyByIdResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetById")
	defer dbSpan.Finish()

	var (
		err  error
		resp pb.Company = pb.Company{}
		stmt            = `SELECT
							id,
							title,
							logo,
							description,
							owner_id
						FROM company
						WHERE id = $1`
	)

	err = c.db.QueryRow(ctx, stmt, id).Scan(
		&resp.Id,
		&resp.Name,
		&resp.Logo,
		&resp.Description,
		&resp.OwnerId,
	)
	if err != nil {
		return nil, err
	}

	return &pb.GetCompanyByIdResponse{
		Company: &resp,
	}, nil
}

func (c *companyRepo) GetList(ctx context.Context, queryParam *pb.GetCompanyListRequest) (*pb.GetComanyListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetById")
	defer dbSpan.Finish()

	var (
		res    = &pb.GetComanyListResponse{}
		params = make(map[string]any)
		arr    []any

		query = `SELECT
			id,
			owner_id,
			title,
			logo,
			description,
			created_at,
			updated_at
		FROM
			"company"`
		filter      = " WHERE is_deleted=false"
		order       = " ORDER BY created_at"
		arrangement = " DESC"
		offset      = " OFFSET 0"
		limit       = " LIMIT 10"
	)

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND ((title || description) ILIKE ('%' || :search || '%'))"
	}

	if len(queryParam.OwnerId) > 0 {
		params["owner_id"] = queryParam.OwnerId
		filter += " AND owner_id = :owner_id"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	var cQ = `SELECT count(1) FROM "company"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)

	if err := c.db.QueryRow(ctx, cQ, arr...).Scan(&res.Count); err != nil {
		return res, err
	}

	var q = query + filter + order + arrangement + offset + limit
	q, arr = helper.ReplaceQueryParams(q, params)

	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			obj       = &pb.Company{}
			createdAt sql.NullString
			updatedAt sql.NullString
		)

		err = rows.Scan(
			&obj.Id,
			&obj.OwnerId,
			&obj.Name,
			&obj.Logo,
			&obj.Description,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return res, err
		}

		res.Companies = append(res.Companies, obj)
	}

	return res, nil
}

func (c *companyRepo) Update(ctx context.Context, company *pb.Company) (*pb.Company, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.Update")
	defer dbSpan.Finish()

	var err error

	if len(company.OwnerId) > 0 {
		updateQuery := `
		UPDATE company
		SET 
		  title = $1,
		  logo = $2,
          description = $3,
		  owner_id = $4,
		  updated_at = current_timestamp
		WHERE id = $5
	`
		_, err = c.db.Exec(ctx, updateQuery,
			company.Name,
			company.Logo,
			company.Description,
			company.OwnerId,
			company.Id,
		)

		if err != nil {
			return nil, err
		}

		resp, err := c.GetById(ctx, company.Id)
		if err != nil {
			return nil, err
		}

		return resp.Company, nil
	}

	updateQuery := `
		UPDATE company
		SET 
		  title = $1,
		  logo = $2,
          description = $3,
		  updated_at = current_timestamp
		WHERE id = $4
	`
	_, err = c.db.Exec(ctx, updateQuery,
		company.Name,
		company.Logo,
		company.Description,
		company.Id,
	)

	if err != nil {
		return nil, err
	}

	resp, err := c.GetById(ctx, company.Id)
	if err != nil {
		return nil, err
	}

	return resp.Company, nil
}

func (c *companyRepo) Delete(ctx context.Context, id string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.Delete")
	defer dbSpan.Finish()

	softDeleteQuery := `
	UPDATE company
	SET
		is_deleted = true,
		deleted_at = CURRENT_TIMESTAMP
	WHERE id = $1
	`
	_, err := c.db.Exec(
		ctx,
		softDeleteQuery,
		id,
	)

	if err != nil {
		return err
	}

	return nil
}

func (c *companyRepo) DeleteAfterTest(ctx context.Context, id string) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.DeleteAfterTest")
	defer dbSpan.Finish()

	deleteQuery := `DELETE FROM company WHERE id = $1`

	result, err := c.db.Exec(ctx, deleteQuery, id)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, nil
}

func (c *companyRepo) GetListWithProjects(ctx context.Context, queryParam *pb.GetListWithProjectsRequest) (*pb.GetListWithProjectsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetListWithProjects")
	defer dbSpan.Finish()

	var (
		res    = &pb.GetListWithProjectsResponse{}
		params = make(map[string]any)

		arr   []any
		query = `SELECT
				c.id,
				c.owner_id,
				c.title,
				c.logo,
				c.description,
				c.created_at,
				c.updated_at
			FROM
				company c`
		filter      = " WHERE c.is_deleted=false"
		order       = " ORDER BY c.created_at"
		arrangement = " DESC"
		offset      = " OFFSET 0"
		limit       = " LIMIT 10"
	)

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND ((c.title || c.description) ILIKE ('%' || :search || '%'))"
	}

	if len(queryParam.OwnerId) > 0 {
		params["owner_id"] = queryParam.OwnerId
		filter += " AND c.owner_id = :owner_id"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	group := ` GROUP BY c.id, p.id`
	projectSoftFilter := ` AND p.is_deleted=false`
	cQ := `SELECT 
			count(1) 
			FROM "company" c
			LEFT JOIN project p
			ON c.id = p.company_id` + filter + projectSoftFilter + group

	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
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
			company          *pb.CompanyWithProjects      = &pb.CompanyWithProjects{}
			projects         []*pb.ProjectWithoutResource = []*pb.ProjectWithoutResource{}
			companyCreatedAt sql.NullString
			companyUpdatedAt sql.NullString
		)

		err = rows.Scan(
			&company.Id,
			&company.OwnerId,
			&company.Name,
			&company.Logo,
			&company.Description,
			&companyCreatedAt,
			&companyUpdatedAt,
		)

		if err != nil {
			return res, err
		}

		query = `SELECT
			id,
			title,
			company_id,
			k8s_namespace,
			created_at,
			updated_at
		FROM
			project
		WHERE is_deleted=false and company_id = $1
		`
		projectRows, err := c.db.Query(ctx, query, company.Id)
		if err != nil {
			return res, err
		}
		defer projectRows.Close()

		for projectRows.Next() {
			var (
				project          *pb.ProjectWithoutResource = &pb.ProjectWithoutResource{}
				projectCreatedAt sql.NullString
				projectUpdatedAt sql.NullString
			)

			err = projectRows.Scan(
				&project.Id,
				&project.Name,
				&project.CompanyId,
				&project.K8SNamespace,
				&projectCreatedAt,
				&projectUpdatedAt,
			)

			if err != nil {
				return res, err
			}

			projects = append(projects, project)
		}

		company.Projects = projects
		res.Companies = append(res.Companies, company)
	}

	return res, nil
}

func (c *companyRepo) GetAllMenuTemplate(ctx context.Context, req *emptypb.Empty) (*pb.GatAllMenuTemplateResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetAllMenuTemplate")
	defer dbSpan.Finish()

	var (
		response = &pb.GatAllMenuTemplateResponse{}
		count    int32
		query    = `SELECT id, coalesce(background, ''), coalesce(active_background, ''), coalesce(text, ''), coalesce(active_text, ''), coalesce(title, '') FROM menu_templates`
	)

	row, err := c.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	for row.Next() {
		var menu pb.MenuTemplate
		err = row.Scan(
			&menu.Id,
			&menu.Background,
			&menu.ActiveBackground,
			&menu.Text,
			&menu.ActiveText,
			&menu.Title,
		)
		if err != nil {
			return nil, err
		}
		response.MenuTemplates = append(response.MenuTemplates, &menu)
	}

	cQuery := `SELECT count(1) FROM menu_templates`
	err = c.db.QueryRow(ctx, cQuery).Scan(&count)
	if err != nil {
		return nil, err
	}
	response.Count = count

	return response, nil
}

func (c *companyRepo) GetMenuTemplateById(ctx context.Context, req *pb.GetMenuTemplateRequest) (*pb.MenuTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetMenuTemplateById")
	defer dbSpan.Finish()

	var response = &pb.MenuTemplate{}

	query := `SELECT id, coalesce(background, ''), coalesce(active_background, ''), coalesce(text, ''), coalesce(active_text, ''), coalesce(title, '') FROM menu_templates WHERE id = $1`
	err := c.db.QueryRow(ctx, query, req.GetId()).Scan(
		&response.Id,
		&response.Background,
		&response.ActiveBackground,
		&response.Text,
		&response.ActiveText,
		&response.Title,
	)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *companyRepo) CreateMenuTemplate(ctx context.Context, req *pb.CreateMenuTemplateRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.CreateMenuTemplate")
	defer dbSpan.Finish()

	query := `INSERT INTO project_menu_templates (id, name, data) VALUES ($1, $2, $3)`
	_, err := c.db.Exec(ctx, query,
		uuid.NewString(),
		req.GetName(),
		req.GetData(),
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *companyRepo) GetProjectMenuTemplates(ctx context.Context, req *pb.GetProjectMenuTemplateRequest) (resp *pb.GetProjectMenuTemplateResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetProjectMenuTemplates")
	defer dbSpan.Finish()

	resp = &pb.GetProjectMenuTemplateResponse{}

	query := `SELECT id, name FROM project_menu_templates`
	rows, err := c.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var menu pb.ProjectMenuTemplate
		err = rows.Scan(
			&menu.Id,
			&menu.Name,
		)
		if err != nil {
			return nil, err
		}
		resp.ProjectMenuTemplates = append(resp.ProjectMenuTemplates, &menu)
	}

	return resp, nil
}
