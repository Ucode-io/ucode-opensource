package postgres

import (
	"context"
	"database/sql"
	"errors"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type docxTemplateRepo struct {
	db *psqlpool.Pool
}

func NewDocxTemplateRepo(db *psqlpool.Pool) storage.DocxTemplateRepoI {
	return &docxTemplateRepo{
		db: db,
	}
}

func (d docxTemplateRepo) Create(ctx context.Context, req *nb.CreateDocxTemplateRequest) (*nb.DocxTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx_template.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	id := uuid.NewString()

	query := `INSERT INTO "docx_templates" (
		id,
		project_id,
		title,
		table_slug,
		file_url,
        pdf_url
	) VALUES ($1, $2, $3, $4, $5, $6)`

	if _, err = tx.Exec(ctx, query, id, req.GetProjectId(), req.GetTitle(), req.GetTableSlug(), req.GetFileUrl(), req.GetPdfUrl()); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return d.GetById(ctx, &nb.DocxTemplatePrimaryKey{Id: id, ProjectId: req.GetProjectId(), ResourceId: req.GetResourceId()})
}

func (d docxTemplateRepo) GetById(ctx context.Context, req *nb.DocxTemplatePrimaryKey) (*nb.DocxTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx_template.GetById")
	defer dbSpan.Finish()
	if req.GetId() == "" || req.GetProjectId() == "" {
		return nil, errors.New("id and project_id cannot be empty")
	}

	var (
		id, projectID, title, tableSlug, fileUrl, pdfUrl sql.NullString
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT
		id,
		project_id,
		title,
		table_slug,
		file_url,
		pdf_url
	FROM "docx_templates" WHERE id = $1 AND project_id = $2`

	if err := conn.QueryRow(ctx, query, req.GetId(), req.GetProjectId()).Scan(&id, &projectID, &title, &tableSlug, &fileUrl, &pdfUrl); err != nil {
		return nil, err
	}

	return &nb.DocxTemplate{
		Id:        id.String,
		ProjectId: projectID.String,
		Title:     title.String,
		TableSlug: tableSlug.String,
		FileUrl:   fileUrl.String,
		PdfUrl:    pdfUrl.String,
	}, nil
}

func (d docxTemplateRepo) GetAll(ctx context.Context, req *nb.GetAllDocxTemplateRequest) (*nb.GetAllDocxTemplateResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx_template.GetAll")
	defer dbSpan.Finish()
	var (
		params = make(map[string]any)
		resp   = &nb.GetAllDocxTemplateResponse{}
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.GetProjectId() == "" {
		return nil, errors.New("project_id cannot be empty")
	}

	params["project_id"] = req.GetProjectId()

	query := `
		SELECT
			id,
			project_id,
			title,
			table_slug,
			file_url,
			pdf_url
		FROM "docx_templates"
		WHERE project_id = :project_id `

	if req.GetTableSlug() != "" {
		query += ` AND table_slug = :table_slug`
		params["table_slug"] = req.GetTableSlug()
	}

	if req.Offset >= 0 {
		query += ` OFFSET :offset `
		params["offset"] = req.Offset
	}

	if req.Limit > 0 {
		query += ` LIMIT :limit `
		params["limit"] = req.Limit
	}

	if req.GetSearch() != "" {
		query += ` AND title ILIKE :search`
		params["search"] = "%" + req.GetSearch() + "%"
	}

	params["project_id"] = req.GetProjectId()

	query, args := helper.ReplaceQueryParams(query, params)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, projectID, title, tableSlug, fileUrl, pdfUrl sql.NullString
		)

		if err = rows.Scan(
			&id,
			&projectID,
			&title,
			&tableSlug,
			&fileUrl,
			&pdfUrl,
		); err != nil {
			return nil, err
		}

		resp.DocxTemplates = append(resp.DocxTemplates, &nb.DocxTemplate{
			Id:        id.String,
			ProjectId: projectID.String,
			Title:     title.String,
			TableSlug: tableSlug.String,
			FileUrl:   fileUrl.String,
			PdfUrl:    pdfUrl.String,
		})
	}

	query = `SELECT COUNT(*) FROM "docx_templates" WHERE project_id = $1`

	if err = conn.QueryRow(ctx, query, req.GetProjectId()).Scan(&resp.Count); err != nil {
		return nil, err
	}

	return resp, nil
}

func (d docxTemplateRepo) Update(ctx context.Context, req *nb.DocxTemplate) (*nb.DocxTemplate, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx_template.Update")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	params := make(map[string]any)
	query := `UPDATE "docx_templates" SET `

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if req.GetId() == "" || req.GetProjectId() == "" {
		return nil, errors.New("id and project_id cannot be empty")
	}

	params["id"] = req.GetId()
	params["project_id"] = req.GetProjectId()

	if req.GetTitle() != "" {
		params["title"] = req.GetTitle()
		query += ` title = :title,`
	}

	if req.GetTableSlug() != "" {
		params["table_slug"] = req.GetTableSlug()
		query += ` table_slug = :table_slug,`
	}

	if req.GetFileUrl() != "" {
		params["file_url"] = req.GetFileUrl()
		query += ` file_url = :file_url,`
	}

	if req.GetPdfUrl() != "" {
		params["pdf_url"] = req.GetPdfUrl()
		query += ` pdf_url = :pdf_url,`
	}

	query = query[:len(query)-1] + ` WHERE id = :id AND project_id = :project_id`

	query, args := helper.ReplaceQueryParams(query, params)

	if _, err = tx.Exec(ctx, query, args...); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return d.GetById(ctx, &nb.DocxTemplatePrimaryKey{Id: req.GetId(), ProjectId: req.GetProjectId(), ResourceId: req.GetResourceId()})
}

func (d docxTemplateRepo) Delete(ctx context.Context, req *nb.DocxTemplatePrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx_template.Delete")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `DELETE from "docx_templates" WHERE id = $1 AND project_id = $2`

	if _, err := conn.Exec(ctx, query, req.GetId(), req.GetProjectId()); err != nil {
		return err
	}

	return nil
}
