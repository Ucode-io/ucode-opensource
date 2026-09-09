package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var (
	table1Slug = "test_table_1"
	table2Slug = "test_table_2"

	table1Label = "Test Table 1"
	table2Label = "Related Table"

	table1Fields = []map[string]any{
		{
			"slug":       "first_name",
			"label":      "First Name",
			"type":       "string",
			"index":      "btree",
			"attributes": json.RawMessage("{}"),
			"is_visible": true,
			"unique":     false,
			"automatic":  false,
			"required":   true,
		},
		{
			"slug":       "last_name",
			"label":      "Last Name",
			"type":       "string",
			"index":      "btree",
			"attributes": json.RawMessage("{}"),
			"is_visible": true,
			"unique":     false,
			"automatic":  false,
			"required":   true,
		},
	}

	table2Fields = []map[string]any{
		{
			"slug":       "name",
			"label":      "Name",
			"type":       "string",
			"index":      "btree",
			"attributes": json.RawMessage("{}"),
			"is_visible": true,
			"unique":     false,
			"automatic":  false,
			"required":   true,
		},
	}
)

// --- Тестовый сценарий (тот же порядок вызовов, что и в storage-версии) ---
func TestItemsFlow(t *testing.T) {

	var (
		mainTableId      = uuid.NewString()
		relatedTableId   = uuid.NewString()
		relatedId        = uuid.NewString()
		mainId           = uuid.NewString()
		multipleUpdateId string
		upsertNewId      string

		err error
	)

	//1) Create main table
	t.Run("1 - Create main table", func(t *testing.T) {
		var (
			tableCreateUrl = BaseUrl + "/v1/table"
			method         = http.MethodPost

			body = map[string]any{
				"slug": table1Slug,
				"id":   mainTableId,
			}
			header = map[string]string{
				"Authorization": AccessToken,
			}
		)

		_, err = UcodeApi.DoRequest(tableCreateUrl, method, body, header)
		assert.NoError(t, err)
	})

	// 2) Create fields for main table
	t.Run("2 - Create fields for main table", func(t *testing.T) {
		var (
			tableCreateUrl = BaseUrl + "/v2/fields/" + table1Slug
			method         = http.MethodPost

			header = map[string]string{
				"Authorization": AccessToken,
			}
		)

		for _, field := range table1Fields {
			field["table_id"] = table1Slug
			field["id"] = uuid.NewString()

			_, err := UcodeApi.DoRequest(tableCreateUrl, method, field, header)
			assert.NoError(t, err)
		}
	})

	// 3) Create related table
	t.Run("3 - Create related table", func(t *testing.T) {
		var (
			tableCreateUrl = BaseUrl + "/v1/table"
			method         = http.MethodPost

			body = map[string]any{
				"slug": table2Slug,
				"id":   relatedTableId,
			}
			header = map[string]string{
				"Authorization": AccessToken,
			}
		)

		_, err = UcodeApi.DoRequest(tableCreateUrl, method, body, header)
		assert.NoError(t, err)
	})

	// 4) Create fields for related table
	t.Run("4 - Create fields for related table", func(t *testing.T) {
		var (
			tableCreateUrl = BaseUrl + "/v2/fields/" + table1Slug
			method         = http.MethodPost

			header = map[string]string{
				"Authorization": AccessToken,
			}
		)

		for _, field := range table2Fields {
			field["table_id"] = table2Slug
			field["id"] = uuid.NewString()

			_, err := UcodeApi.DoRequest(tableCreateUrl, method, field, header)
			assert.NoError(t, err)
		}
	})

	// 5) Create relation for main table
	t.Run("5 - Create relation for main table", func(t *testing.T) {
		var (
			tableCreateUrl = BaseUrl + "/v2/relations/" + table1Slug
			method         = http.MethodPost

			body = map[string]any{
				"table_from": table1Slug,
				"table_to":   table2Slug,
				"required":   false,
				"show_label": true,
				"type":       "Many2One",
			}

			header = map[string]string{
				"Authorization": AccessToken,
			}
		)

		_, err = UcodeApi.DoRequest(tableCreateUrl, method, body, header)
		assert.NoError(t, err)
	})

	// 6) Create related item (will be referenced via relation field)
	t.Run("6 - Create related item", func(t *testing.T) {

		_, _, err = UcodeApi.Items(table2Slug).Create(
			map[string]any{
				"from_auth_service": false,
				"guid":              relatedId,
				"name":              fakeData.Name(),
			},
		).Exec()
		assert.NoError(t, err)
	})

	// 7) Create main item with relation to relatedId
	t.Run("7 - Create main item with relation", func(t *testing.T) {

		_, _, err = UcodeApi.Items(table1Slug).Create(
			map[string]any{
				"from_auth_service": false,
				"guid":              mainId,
				"first_name":        fakeData.FirstName(),
				"last_name":         fakeData.LastName(),
				"test_table_2_id":   relatedId,
			},
		).Exec()
		assert.NoError(t, err)
	})

	// 8) Get single
	t.Run("8 - Get single item", func(t *testing.T) {
		_, _, err = UcodeApi.Items(table1Slug).GetSingle(mainId).Exec()
		assert.NoError(t, err)
	})

	// 9) Update main item
	t.Run("9 - Update main item", func(t *testing.T) {

		_, _, err = UcodeApi.Items(table1Slug).Update(
			map[string]any{
				"guid":       mainId,
				"first_name": fakeData.FirstName(),
			},
		).ExecSingle()
		assert.NoError(t, err)
	})

	// 10) MultipleUpdate: update existing and add new
	t.Run("10 - MultipleUpdate (update existing and add new)", func(t *testing.T) {
		multipleUpdateId = uuid.NewString()

		objects := []map[string]any{
			{
				"guid":       mainId,
				"first_name": fakeData.FirstName(),
				"last_name":  fakeData.LastName(),
				"is_new":     false,
			},
			{
				"first_name": fakeData.FirstName(),
				"last_name":  fakeData.LastName(),
				"is_new":     true,
			},
		}
		_, _, err = UcodeApi.Items(table1Slug).Update(map[string]any{"objects": objects}).ExecMultiple()
		assert.NoError(t, err)
	})

	// 11) UpsertMany
	t.Run("11 - UpsertMany", func(t *testing.T) {

		upsertNewId = uuid.NewString()

		var (
			tableCreateUrl = BaseUrl + "/v2/items/" + table1Slug + "/upsert-many"
			method         = http.MethodPost

			header = map[string]string{
				"Authorization": AccessToken,
			}

			upsertObjects = []map[string]any{
				{
					"guid":       mainId,
					"first_name": fakeData.FirstName(),
					"last_name":  fakeData.LastName(),
				},
				{
					"guid":       upsertNewId,
					"first_name": fakeData.FirstName(),
					"last_name":  fakeData.LastName(),
				},
			}

			dataMap = map[string]any{
				"field_slug": "guid",
				"fields":     []string{"guid", "first_name", "last_name"},
				"objects":    upsertObjects,
			}
		)

		_, err = UcodeApi.DoRequest(tableCreateUrl, method, map[string]any{"data": dataMap}, header)
		assert.NoError(t, err)
	})

	// 12) Get list 2
	t.Run("12 - Getlist2", func(t *testing.T) {

		_, _, err = UcodeApi.Items(table1Slug).GetList().Exec()
		assert.NoError(t, err)
	})

	// 13) Get list aggregation
	t.Run("13 - GetList aggregation", func(t *testing.T) {

		var pipeline = map[string]any{
			"operation": "SELECT",
			"table":     table1Slug,
			"columns":   []string{"guid", "first_name", "last_name"},
			"limit":     1,
		}

		_, _, err = UcodeApi.Items(table1Slug).GetList().Pipelines(pipeline).ExecAggregation()
		assert.NoError(t, err)
	})

	// 14) DeleteMany (remove existing and the one created by MultipleUpdate)
	t.Run("14 - DeleteMany", func(t *testing.T) {
		_, err = UcodeApi.Items(table1Slug).Delete().Multiple([]string{mainId, multipleUpdateId, upsertNewId}).Exec()
		assert.NoError(t, err)
	})

	// 15) Delete single (remove the one created by UpsertMany)
	t.Run("15 - Delete single", func(t *testing.T) {
		_, err = UcodeApi.Items(table2Slug).Delete().Single(relatedId).Exec()
		assert.NoError(t, err)
	})

	// 16) Delete tables
	t.Run("16 - Delete tables", func(t *testing.T) {

		var (
			tableDeleteUrl = BaseUrl + "/v1/table/" + mainTableId
			method         = http.MethodDelete

			header = map[string]string{
				"Authorization": AccessToken,
			}
		)

		_, err = UcodeApi.DoRequest(tableDeleteUrl, method, nil, header)
		assert.NoError(t, err)

		tableDeleteUrl = BaseUrl + "/v1/table/" + relatedTableId

		_, err = UcodeApi.DoRequest(tableDeleteUrl, method, nil, header)
		assert.NoError(t, err)

	})
}
