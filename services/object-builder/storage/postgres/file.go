package postgres

import (
	"context"
	"fmt"
	"strings"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type fileRepo struct {
	db *psqlpool.Pool
}

func NewFileRepo(db *psqlpool.Pool) storage.FileRepoI {
	return &fileRepo{
		db: db,
	}
}

func (f *fileRepo) Create(ctx context.Context, req *nb.CreateFileRequest) (resp *nb.File, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "file.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	fileId := uuid.NewString()
	storageType := strings.TrimSpace(req.GetStorageType())
	if storageType == "" {
		storageType = "minio"
	}

	query := `INSERT INTO "file" (
		"id",
		"title",
		"description",
		"tags",
		"storage",
		"storage_type",
		"file_name_disk", 
		"file_name_download",
		"link",
		"file_size"
	) VALUES ($1, $2, $3, $4, $5, $6::file_storage_type, $7, $8, $9, $10)`

	_, err = conn.Exec(ctx, query,
		fileId,
		req.Title,
		req.Description,
		req.Tags,
		req.Storage,
		storageType,
		req.FileNameDisk,
		req.FileNameDownload,
		req.Link,
		req.FileSize,
	)
	if err != nil {
		return &nb.File{}, err
	}

	return &nb.File{
		Id:               fileId,
		Title:            req.Title,
		Description:      req.Description,
		Tags:             req.Tags,
		Storage:          req.Storage,
		StorageType:      storageType,
		FileNameDisk:     req.FileNameDisk,
		FileNameDownload: req.FileNameDownload,
		Link:             req.Link,
		FileSize:         req.FileSize,
		ProjectId:        req.ProjectId,
	}, nil
}

func (f *fileRepo) GetSingle(ctx context.Context, req *nb.FilePrimaryKey) (resp *nb.File, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "file.GetSingle")
	defer dbSpan.Finish()

	resp = &nb.File{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT 
				"id",
				"title",
				"description",
				"tags",
				"storage",
				"storage_type",
				"file_name_disk", 
				"file_name_download",
				"link",
				"file_size"
		FROM "file" WHERE id = $1`

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&resp.Title,
		&resp.Description,
		&resp.Tags,
		&resp.Storage,
		&resp.StorageType,
		&resp.FileNameDisk,
		&resp.FileNameDownload,
		&resp.Link,
		&resp.FileSize,
	)
	if err != nil {
		return &nb.File{}, err
	}

	return resp, nil
}

func (f *fileRepo) GetList(ctx context.Context, req *nb.GetAllFilesRequest) (resp *nb.GetAllFilesResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "file.GetList")
	defer dbSpan.Finish()

	resp = &nb.GetAllFilesResponse{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT 
				COUNT(*) OVER(),
				"id",
				"title",
				"description",
				"tags",
				"storage",
				"storage_type",
				"file_name_disk", 
				"file_name_download",
				"link",
				"file_size"
			FROM "file"  
			WHERE 1=1
	`

	if req.Search != "" {
		query += fmt.Sprintf(` AND title ~* '%s'`, req.Search)
	}

	if req.FolderName != "" {
		query += fmt.Sprintf(` AND storage = '%s'`, req.FolderName)
	}

	if req.Sort != "" {
		query += ` ORDER BY created_at ` + req.GetSort()
	}

	if req.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d `, req.Limit)
	}

	if req.Offset > 0 {
		query += fmt.Sprintf(` OFFSET %d `, req.Offset)
	}

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		row := &nb.File{}

		err = rows.Scan(
			&resp.Count,
			&row.Id,
			&row.Title,
			&row.Description,
			&row.Tags,
			&row.Storage,
			&row.StorageType,
			&row.FileNameDisk,
			&row.FileNameDownload,
			&row.Link,
			&row.FileSize,
		)
		if err != nil {
			return resp, err
		}

		resp.Files = append(resp.Files, row)
	}

	return resp, nil
}

func (f *fileRepo) Update(ctx context.Context, req *nb.File) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "file.Update")
	defer dbSpan.Finish()

	var (
		query = `UPDATE "file" SET
				"title" = $2,
				"description" = $3,
				"tags" = $4,
				"file_name_download" = $5,
				"updated_at" = CURRENT_TIMESTAMP
			WHERE id = $1`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, query,
		req.Id,
		req.Title,
		req.Description,
		req.Tags,
		req.FileNameDownload,
	); err != nil {
		return err
	}

	return nil
}

func (f *fileRepo) Delete(ctx context.Context, req *nb.FileDeleteRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "file.Delete")
	defer dbSpan.Finish()

	var (
		query = ` DELETE FROM "file" WHERE id = ANY($1)`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	_, err = conn.Exec(ctx, query, pq.Array(req.Ids))
	if err != nil {
		return errors.Wrap(err, "Delete")
	}
	return nil
}
