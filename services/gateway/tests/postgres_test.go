package tests

//import (
//	"encoding/json"
//	"testing"
//	"time"
//
//	"github.com/google/uuid"
//	"github.com/manveru/faker"
//	"github.com/stretchr/testify/assert"
//)
//
//var (
//	baseUrl = "https://api.admin.u-code.io"
//)
//
//func TestEndToEnd(t *testing.T) {
//	faker1, err := faker.New("en")
//	assert.NoError(t, err, "faker error")
//	token, err := Login()
//	// assert.NoError(t, err)
//	// assert.NotEmpty(t, token)
//
//	t.Run("CRUDOperations", func(t *testing.T) {
//		guid := uuid.New().String()
//		body := map[string]any{
//			"guid":                             guid,
//			"single_line_field":                faker1.CompanyName(),
//			"multi_line_field":                 faker1.FirstName(),
//			"email_field":                      faker1.Email(),
//			"internation_phone_field":          faker1.PhoneNumber(),
//			"date_time_field":                  time.Now(),
//			"date_time_without_timezone_field": time.Now().UTC(),
//			"date_field":                       time.Now(),
//		}
//		_, _, err = UcodeApiForStaging.Items("product").
//			Create(body).
//			DisableFaas(true).
//			Exec()
//		assert.NoError(t, err)
//
//		body = map[string]any{
//			"guid":             guid,
//			"multi_line_field": faker1.Name(),
//			"checkbox_field":   true,
//		}
//
//		_, _, err = UcodeApiForStaging.Items("product").
//			Update(body).
//			DisableFaas(true).
//			ExecSingle()
//		assert.NoError(t, err)
//
//		slimResponse, _, err := UcodeApiForStaging.Items("product").GetSingle(guid).ExecSlim()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, slimResponse.Data.Data.Response, "item response should not be empty")
//
//		itemResponse, _, err := UcodeApiForStaging.Items("product").GetSingle(guid).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, itemResponse.Data.Data.Response, "item response should not be empty")
//
//		_, err = UcodeApiForStaging.Items("product").Delete().DisableFaas(true).Single(guid).Exec()
//		assert.NoError(t, err)
//	})
//
//	t.Run("GetListSlim", func(t *testing.T) {
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Limit(10).Page(1).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, "response not equal to limit")
//	})
//
//	t.Run("Pagination", func(t *testing.T) {
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Limit(5).Page(4).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, "response not equal to limit")
//
//		getProductResp, _, err = UcodeApiForStaging.Items("product").GetList().Limit(5).Page(5).Exec()
//		assert.NoError(t, err)
//		assert.Empty(t, getProductResp.Data.Data.Response, "response should be empty")
//	})
//
//	t.Run("TestGetListSlimWithDate", func(t *testing.T) {
//		filter := map[string]any{
//			"date_time_field": map[string]any{
//				"$gte": "2024-10-01T00:04:19.336Z",
//			}}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Filter(filter).Page(1).Limit(10).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, err)
//
//		filter = map[string]any{
//			"date_time_field": map[string]any{
//				"$gte": "2024-10-01T00:04:19.336Z",
//				"$lt":  "2024-10-06T00:04:19.336Z",
//			},
//		}
//
//		getProductResp, _, err = UcodeApi.Items("product").GetList().Limit(10).Page(1).Filter(filter).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, err)
//	})
//
//	t.Run("TestGetListSlimWithEq", func(t *testing.T) {
//		filter := map[string]any{
//			"increment_id_field": map[string]any{
//				"$eq": "T-000000021",
//			}}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Filter(filter).Page(1).Limit(10).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, err)
//	})
//
//	t.Run("TestGetListSlimWithIn", func(t *testing.T) {
//		filter := map[string]any{
//			"increment_id_field": map[string]any{
//				"$in": []string{"T-000000022", "T-000000023", "T-000000024"},
//			}}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Filter(filter).Page(1).Limit(10).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, err)
//	})
//
//	t.Run("TestGetListSlimWithBoolean", func(t *testing.T) {
//		filter := map[string]any{
//			"switch_field": false,
//		}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Filter(filter).Page(1).Limit(10).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, err)
//	})
//
//	t.Run("TestGetListObject", func(t *testing.T) {
//		filter := map[string]any{}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Filter(filter).Page(1).Limit(10).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, err)
//	})
//
//	t.Run("TestGetListObjectSearch", func(t *testing.T) {
//		filter := map[string]any{
//			"search":      "+99894",
//			"view_fields": []string{"internation_phone_field"},
//		}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").GetList().Filter(filter).Page(1).Limit(10).Exec()
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Response, "Switch Does not work")
//
//		filter = map[string]any{
//			"search":      "+4",
//			"view_fields": []string{"internation_phone_field"},
//		}
//
//		getProductResp, _, err = UcodeApi.Items("product").GetList().Limit(10).Page(1).Filter(filter).Exec()
//		assert.NoError(t, err)
//		assert.Empty(t, getProductResp.Data.Data.Response, "Response should be empty")
//	})
//
//	t.Run("TestGetAggregation", func(t *testing.T) {
//		filter := map[string]any{
//			"pipelines": []map[string]any{
//				{"$group": map[string]any{"_id": "$single_line_field"}},
//			},
//		}
//		getProductResp, _, err := UcodeApiForStaging.Items("product").
//			GetList().
//			Pipelines(filter).
//			ExecAggregation()
//
//		assert.NoError(t, err)
//		assert.NotEmpty(t, getProductResp.Data.Data.Data, "Switch Does not work")
//	})
//
//	t.Run("TestGetListAutoFilter", func(t *testing.T) {
//		assert.NoError(t, err)
//
//		body, err := UcodeApiForStaging.DoRequest(BaseUrlStaging+"/v2/object/get-list/product", "POST", map[string]any{
//			"data": map[string]any{},
//		}, map[string]string{
//			"Authorization": "Bearer " + token,
//		})
//		assert.NoError(t, err)
//		var response GetListApiResponse
//
//		err = json.Unmarshal(body, &response)
//		assert.NoError(t, err)
//
//		assert.NotEmpty(t, response.Data.Data.Response)
//	})
//
//	t.Run("TestGetListExcel", func(t *testing.T) {
//		body, err := UcodeApi.DoRequest(BaseUrlStaging+"/v1/object/excel/product", "POST", ExcelReq, map[string]string{
//			"authorization": "API-KEY",
//			"X-API-KEY":     UcodeApiForStaging.Config().AppId,
//		})
//		assert.NoError(t, err)
//		var response ExcelResponse
//		err = json.Unmarshal(body, &response)
//		assert.NoError(t, err)
//
//		assert.NotEmpty(t, response.Data.Data.Link)
//	})
//
//	t.Run("TestGetListRBAC", func(t *testing.T) {
//		assert.NoError(t, err)
//
//		body, err := UcodeApi.DoRequest(BaseUrlStaging+"/v2/object/get-list/company", "POST", map[string]any{
//			"data": map[string]any{},
//		}, map[string]string{
//			"Authorization": "Bearer " + token,
//		})
//		assert.NoError(t, err)
//		var response GetListApiResponse
//
//		err = json.Unmarshal(body, &response)
//		assert.NoError(t, err)
//
//		assert.NotEmpty(t, response.Data.Data.Response)
//	})
//
//	t.Run("TestMultipleCRUD", func(t *testing.T) {
//		var ids = []string{}
//		var multipleInsert = []map[string]any{}
//		var multipleUpdate = []map[string]any{}
//		UcodeApiForStaging.Config().RequestTimeout = time.Second * 30
//
//		for i := 0; i < 10; i++ {
//			guid := uuid.New().String()
//			multipleInsert = append(multipleInsert, map[string]any{
//				"guid":                             guid,
//				"single_line_field":                faker1.CompanyName(),
//				"multi_line_field":                 faker1.FirstName(),
//				"email_field":                      faker1.Email(),
//				"internation_phone_field":          faker1.PhoneNumber(),
//				"date_time_field":                  time.Now(),
//				"date_time_without_timezone_field": time.Now().UTC(),
//				"date_field":                       time.Now(),
//				"is_new":                           true,
//			})
//
//			multipleUpdate = append(multipleUpdate, map[string]any{
//				"guid":                             guid,
//				"single_line_field":                faker1.CompanyName(),
//				"multi_line_field":                 faker1.FirstName(),
//				"email_field":                      faker1.Email(),
//				"internation_phone_field":          faker1.PhoneNumber(),
//				"date_time_field":                  time.Now(),
//				"date_time_without_timezone_field": time.Now().UTC(),
//				"date_field":                       time.Now(),
//				"is_new":                           false,
//			})
//			ids = append(ids, guid)
//		}
//
//		_, _, err = UcodeApiForStaging.Items("product").
//			Update(map[string]any{"objects": multipleInsert}).
//			DisableFaas(true).
//			ExecMultiple()
//		assert.NoError(t, err)
//
//		_, _, err = UcodeApiForStaging.Items("product").
//			Update(map[string]any{"objects": multipleUpdate}).
//			DisableFaas(true).
//			ExecMultiple()
//		assert.NoError(t, err)
//
//		_, err = UcodeApiForStaging.Items("product").
//			Delete().
//			DisableFaas(true).DisableFaas(true).Multiple(ids).Exec()
//		assert.NoError(t, err)
//	})
//
//}
