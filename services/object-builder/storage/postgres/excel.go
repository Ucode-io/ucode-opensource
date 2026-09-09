package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/tealeg/xlsx"
	"github.com/xuri/excelize/v2"
)

type excelRepo struct {
	db *psqlpool.Pool
}

func NewExcelRepo(db *psqlpool.Pool) storage.ExcelRepoI {
	return &excelRepo{
		db: db,
	}
}

func (e *excelRepo) ExcelRead(ctx context.Context, req *nb.ExcelReadRequest) (resp *nb.ExcelReadResponse, err error) {
	dbSpan, _ := opentracing.StartSpanFromContext(ctx, "excel.ExcelRead")
	defer dbSpan.Finish()
	cfg := config.Load()

	endpoint := cfg.MinioHost
	accessKeyID := cfg.MinioAccessKeyID
	secretAccessKey := cfg.MinioSecretKey
	fileID := req.Id

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.MinioSSL,
	})
	if err != nil {
		return &nb.ExcelReadResponse{}, err
	}

	bucketName := req.ProjectId
	fileObjectKey := fileID + ".xlsx"
	createFilePath := "./" + fileObjectKey

	err = downloadFile(minioClient, bucketName, fileObjectKey, createFilePath)
	if err != nil {
		return &nb.ExcelReadResponse{}, err
	}

	objectRow, err := readFirstRow(createFilePath)
	if err != nil {
		return &nb.ExcelReadResponse{}, err
	}

	err = os.Remove(createFilePath)
	if err != nil {
		return &nb.ExcelReadResponse{}, err
	}

	return &nb.ExcelReadResponse{Rows: objectRow}, nil
}

func (e *excelRepo) ExcelToDb(ctx context.Context, req *nb.ExcelToDbRequest) (resp *nb.ExcelToDbResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "excel.ExcelToDb")
	defer dbSpan.Finish()
	var (
		cfg             = config.Load()
		endpoint        = cfg.MinioHost
		accessKeyID     = cfg.MinioAccessKeyID
		secretAccessKey = cfg.MinioSecretKey
		i               int
		fieldsMap       = make(map[string]models.Field)
		slugsMap        = make(map[string]string)
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "conn.Begin")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.MinioSSL,
	})
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "minio.New")
	}

	object, err := minioClient.GetObject(ctx, req.ProjectId, req.Id+".xlsx", minio.GetObjectOptions{})
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "minioClient.GetObject")
	}
	defer object.Close()

	localFileName := "localfile.xlsx"
	localFile, err := os.Create(localFileName)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "os.Create")
	}
	defer localFile.Close()

	if _, err := io.Copy(localFile, object); err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "io.Copy")
	}

	f, err := excelize.OpenFile(localFileName)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "excelize.OpenFile")
	}

	query := `SELECT f.id, f.slug, f.type FROM "field" f JOIN "table" t ON f.table_id = t.id WHERE t.slug = $1`

	fieldRows, err := tx.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "tx.Query")
	}
	defer fieldRows.Close()

	data, err := helper.ConvertStructToMap(req.Data)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "ConvertStructToMap")
	}

	fields := []models.Field{}

	for fieldRows.Next() {
		var (
			id, slug, ftype string
		)

		err = fieldRows.Scan(
			&id,
			&slug,
			&ftype,
		)
		if err != nil {
			return &nb.ExcelToDbResponse{}, errors.Wrap(err, "fieldRows.Scan")
		}

		fieldsMap[id] = models.Field{
			Id:   id,
			Slug: slug,
			Type: ftype,
		}
		fieldsMap[slug] = models.Field{
			Id:   id,
			Slug: slug,
			Type: ftype,
		}
		fields = append(fields, models.Field{
			Id:   id,
			Slug: slug,
			Type: ftype,
		})
	}

	sheetlist := f.GetSheetList()
	if len(sheetlist) == 0 {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "No sheets found")
	}

	sheetName := sheetlist[0]

	for {
		cell, err := f.GetCellValue(sheetName, convertToTitle(i)+"1")
		if err != nil {
			return &nb.ExcelToDbResponse{}, errors.Wrap(err, "GetCellValue")
		}
		if cell == "" {
			break
		}

		fieldId := cast.ToString(data[cell])
		slug := fieldsMap[fieldId]

		slugsMap[convertToTitle(i)] = slug.Slug
		i++
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "GetRows")
	}

	var fullData = []map[string]any{}

	for c, row := range rows {
		if c == 0 {
			continue
		}

		body := make(map[string]any)

		for i, cell := range row {
			var value any
			if cell != "" {
				field := fieldsMap[slugsMap[convertToTitle(i)]]
				if helper.FIELD_TYPES[field.Type] == "FLOAT" {
					value = cast.ToInt(cell)
				} else if field.Type == "MULTISELECT" {
					value = strings.Split(cell, ",")
				} else if field.Type == "SWITCH" || field.Type == "CHECKBOX" {
					if strings.ToUpper(cell) == "ИСТИНА" || strings.ToUpper(cell) == "TRUE" {
						value = true
					} else {
						value = false
					}
				} else {
					value = cell
				}

				body[slugsMap[convertToTitle(i)]] = value
			}
		}
		fullData = append(fullData, body)
	}

	query, args, err := MakeQueryForMultiInsert(ctx, tx, req.TableSlug, fullData, fields)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "MakeQueryForMultiInsert")
	}

	rowsReturned, err := tx.Query(ctx, query+" RETURNING *", args...)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "tx.Query RETURNING")
	}
	defer rowsReturned.Close()

	var insertedRows []map[string]any
	for rowsReturned.Next() {
		values, err := rowsReturned.Values()
		if err != nil {
			return &nb.ExcelToDbResponse{}, errors.Wrap(err, "rowsReturned.Values")
		}
		fds := rowsReturned.FieldDescriptions()
		rowMap := make(map[string]any, len(values))
		for idx, fd := range fds {
			if value, ok := values[idx].([16]uint8); ok { // uuid
				rowMap[string(fd.Name)] = uuid.UUID(value).String()
				continue
			}

			if idx < len(values) {
				rowMap[string(fd.Name)] = values[idx]
			}
		}

		insertedRows = append(insertedRows, rowMap)
	}

	newResp, err := helper.Convert[[]map[string]any, []*structpb.Struct](insertedRows)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "error while converting map to struct")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &nb.ExcelToDbResponse{}, errors.Wrap(err, "tx.Commit")
	}

	return &nb.ExcelToDbResponse{Rows: newResp}, nil
}

func MakeQueryForMultiInsert(ctx context.Context, tx pgx.Tx, tableSlug string, data []map[string]any, fields []models.Field) (string, []any, error) {
	var (
		args       []any
		argCount   = 1
		tableSlugs = []string{}
		fieldM     = make(map[string]models.FieldBody)
		newFields  = []models.Field{}
		query      = fmt.Sprintf(`INSERT INTO %s (`, tableSlug)
	)

	for index, field := range fields {
		if field.Slug == "guid" || field.Type == "INCREMENT_NUMBER" {
			continue
		}

		if index == len(fields)-1 {
			query += field.Slug
			break
		}

		query += fmt.Sprintf("%s, ", field.Slug)
	}

	query += ") VALUES"

	fQuery := ` SELECT
		f."id",
		f."type",
		f."attributes",
		f."relation_id",
		f."autofill_table",
		f."autofill_field",
		f."slug"
	FROM "field" f JOIN "table" as t ON f.table_id = t.id WHERE t.slug = $1`

	fieldRows, err := tx.Query(ctx, fQuery, tableSlug)
	if err != nil {
		return "", nil, err
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		field := models.Field{}

		var (
			atr           = []byte{}
			autoFillTable sql.NullString
			autoFillField sql.NullString
			relationId    sql.NullString
			attributes    = make(map[string]any)
		)

		err = fieldRows.Scan(
			&field.Id,
			&field.Type,
			&atr,
			&relationId,
			&autoFillTable,
			&autoFillField,
			&field.Slug,
		)
		if err != nil {
			return "", nil, err
		}

		if err := json.Unmarshal(atr, &field.Attributes); err != nil {
			return "", nil, err
		}
		if err := json.Unmarshal(atr, &attributes); err != nil {
			return "", nil, err
		}

		tableSlugs = append(tableSlugs, field.Slug)

		if config.Ftype[field.Type] {
			fieldM[field.Type] = models.FieldBody{
				Slug:       field.Slug,
				Attributes: attributes,
			}
		}

		newFields = append(newFields, field)
	}

	reqBody := models.CreateBody{
		FieldMap:   fieldM,
		Fields:     newFields,
		TableSlugs: tableSlugs,
	}

	for _, body := range data {
		structBody, err := helper.ConvertMapToStruct(body)
		if err != nil {
			return "", nil, err
		}

		body, _, err = helper.PrepareToCreateInObjectBuilder(ctx, tx, &nb.CommonMessage{
			Data:      structBody,
			TableSlug: tableSlug,
		}, reqBody)
		if err != nil {
			return "", nil, err
		}

		query += " ("
		for _, field := range fields {
			if field.Type == "INCREMENT_NUMBER" || field.Slug == "guid" {
				continue
			}

			query += fmt.Sprintf(" $%d,", argCount)
			args = append(args, body[field.Slug])
			argCount++
		}

		query = strings.TrimRight(query, ",")
		query += "),"
	}

	query = strings.TrimRight(query, ",")

	return query, args, nil
}

func downloadFile(minioClient *minio.Client, bucketName, fileObjectKey, createFilePath string) error {
	object, err := minioClient.GetObject(context.Background(), bucketName, fileObjectKey, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer object.Close()

	file, err := os.Create(createFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, object)
	if err != nil {
		return fmt.Errorf("error copying object to file: %v", err)
	}

	return nil
}

func readFirstRow(filePath string) ([]string, error) {
	xlFile, err := xlsx.OpenFile(filePath)
	if err != nil {
		return nil, err
	}

	var firstRow []string
	if len(xlFile.Sheets) > 0 {
		firstSheet := xlFile.Sheets[0]
		if len(firstSheet.Rows) > 0 {
			for _, cell := range firstSheet.Rows[0].Cells {
				firstRow = append(firstRow, cell.String())
			}
		}
	}

	return firstRow, nil
}
