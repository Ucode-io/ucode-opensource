package postgres

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/opentracing/opentracing-go"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type csvRepo struct {
	db *psqlpool.Pool
}

func NewCSVRepo(db *psqlpool.Pool) storage.CSVRepoI {
	return &csvRepo{
		db: db,
	}
}

func (o *csvRepo) GetListInCSV(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "csv.GetListInCSV")
	defer dbSpan.Finish()

	var (
		params    = make(map[string]any)
		fields    = make(map[string]models.Field)
		fieldsArr = []models.Field{}
		fieldIds  = cast.ToStringSlice(params["field_ids"])
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	if err := json.Unmarshal(paramBody, &params); err != nil {
		return &nb.CommonMessage{}, err
	}

	delete(params, "field_ids")

	query := `SELECT f.type, f.slug, f.attributes, f.label FROM "field" f WHERE f.id = ANY ($1)`

	fieldRows, err := conn.Query(ctx, query, pq.Array(fieldIds))
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			fBody = models.Field{}
			attrb = []byte{}
		)

		err = fieldRows.Scan(
			&fBody.Type,
			&fBody.Slug,
			&attrb,
			&fBody.Label,
		)
		if err != nil {
			return &nb.CommonMessage{}, err
		}

		if err := json.Unmarshal(attrb, &fBody.Attributes); err != nil {
			return &nb.CommonMessage{}, err
		}

		fields[fBody.Slug] = fBody
		fieldsArr = append(fieldsArr, fBody)
	}

	items, _, err := helper.GetItems(ctx, conn, models.GetItemsBody{
		TableSlug: req.TableSlug,
		Params:    params,
		FieldsMap: fields,
	})
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	curDir, err := os.Getwd()
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	filename := fmt.Sprintf("report_%d.csv", time.Now().Unix())
	filepath := curDir + "/" + filename
	file, err := os.Create(filepath)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	headers := []string{}
	for _, field := range fieldsArr {
		headers = append(headers, field.Label)
	}
	if err := writer.Write(headers); err != nil {
		return &nb.CommonMessage{}, err
	}

	for _, item := range items {
		record := []string{}
		for _, field := range fieldsArr {
			switch field.Type {
			case "MULTI_LINE":
				re := regexp.MustCompile(`<[^>]+>`)
				result := re.ReplaceAllString(cast.ToString(item[field.Slug]), "")
				item[field.Slug] = result
			case "DATE":
				timeF, err := time.Parse("2006-01-02", strings.Split(cast.ToString(item[field.Slug]), " ")[0])
				if err != nil {
					return &nb.CommonMessage{}, err
				}
				item[field.Slug] = timeF.Format("02.01.2006")
			case "DATE_TIME":
				newTime := strings.Split(cast.ToString(item[field.Slug]), " ")[0] + " " + strings.Split(cast.ToString(item[field.Slug]), " ")[1]
				timeF, err := time.Parse("2006-01-02 15:04:05", newTime)
				if err != nil {
					return &nb.CommonMessage{}, err
				}
				item[field.Slug] = timeF.Format("02.01.2006 15:04")
			case "MULTISELECT":
				attributes, err := helper.ConvertStructToMap(field.Attributes)
				if err != nil {
					return &nb.CommonMessage{}, err
				}
				multiselectValue := ""
				if options, ok := attributes["options"].([]any); ok {
					values := cast.ToStringSlice(item[field.Slug])
					for _, val := range values {
						for _, op := range options {
							opt := cast.ToStringMap(op)
							if val == cast.ToString(opt["value"]) {
								if label, ok := opt["label"].(string); ok && label != "" {
									multiselectValue += label + ","
								} else {
									multiselectValue += cast.ToString(opt["value"]) + ","
								}
							}
						}
					}
				}
				item[field.Slug] = strings.TrimRight(multiselectValue, ",")
			}
			record = append(record, cast.ToString(item[field.Slug]))
		}
		if err := writer.Write(record); err != nil {
			return &nb.CommonMessage{}, err
		}
	}

	writer.Flush()

	cfg := config.Load()

	endpoint := cfg.MinioHost
	accessKeyID := cfg.MinioAccessKeyID
	secretAccessKey := cfg.MinioSecretKey

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.MinioSSL,
	})
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	_, err = minioClient.FPutObject(
		context.Background(),
		"reports",
		filename,
		filepath,
		minio.PutObjectOptions{ContentType: "text/csv"},
	)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	err = os.Remove(filepath)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	link := fmt.Sprintf("%s/reports/%s", endpoint, filename)
	respCSV := map[string]string{
		"link": link,
	}

	marshaledInputMap, err := json.Marshal(respCSV)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	err = protojson.Unmarshal(marshaledInputMap, outputStruct)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	return &nb.CommonMessage{TableSlug: req.TableSlug, Data: outputStruct}, nil
}
