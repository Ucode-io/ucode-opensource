package postgres_test

import (
	"context"
	"fmt"
	"testing"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func createTable(t *testing.T, slug, label string) (string, error) {
	req := &nb.CreateTableRequest{
		Label:     label,
		Slug:      slug,
		ProjectId: projectId,
	}

	resp, err := strg.Table().Create(context.Background(), req)
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

func createRelation(t *testing.T, tableFrom, tableTo string) error {
	req := &nb.CreateRelationRequest{
		TableFrom:       tableFrom,
		TableTo:         tableTo,
		Type:            "Many2One",
		ViewFields:      []string{},
		RelationFieldId: uuid.New().String(),
		ProjectId:       projectId,
		Attributes:      &structpb.Struct{},
	}

	_, err = strg.Relation().Create(context.Background(), req)
	return err
}

func createFields(t *testing.T, tableId string, fields []map[string]any) error {
	for _, f := range fields {
		req := &nb.CreateFieldRequest{
			Id:            uuid.New().String(),
			ProjectId:     projectId,
			TableId:       tableId,
			Slug:          fmt.Sprintf("%v", f["slug"]),
			Label:         fmt.Sprintf("%v", f["label"]),
			Type:          fmt.Sprintf("%v", f["type"]),
			Index:         fmt.Sprintf("%v", f["index"]),
			IsVisible:     f["is_visible"].(bool),
			Unique:        f["unique"].(bool),
			AutofillField: fmt.Sprintf("%v", f["automatic"]),
			Attributes:    &structpb.Struct{},
		}

		_, err := strg.Field().Create(context.Background(), req)
		return err
	}

	return nil
}

func createItem(t *testing.T, tableSlug string, dataMap map[string]any) (string, error) {
	id := uuid.NewString()
	if dataMap == nil {
		dataMap = map[string]any{
			"from_auth_service": false,
			"guid":              id,
			"first_name":        fakeData.Person().FirstName(),
			"last_name":         fakeData.Person().LastName(),
		}
	} else {
		if _, ok := dataMap["guid"]; !ok {
			dataMap["guid"] = id
		}
	}

	data, err := helper.ConvertMapToStruct(dataMap)
	if err != nil {
		return "", err
	}

	req := &nb.CommonMessage{
		ProjectId: projectId,
		TableSlug: tableSlug,
		Data:      data,
	}

	_, err = strg.Items().Create(context.Background(), req)
	return id, err
}

func getSingleItem(t *testing.T, tableSlug, id string) error {

	dataMap := map[string]any{
		"id": id,
	}

	data, err := helper.ConvertMapToStruct(dataMap)
	if err != nil {
		return err
	}

	req := &nb.CommonMessage{
		ProjectId: projectId,
		TableSlug: tableSlug,
		Data:      data,
	}

	_, err = strg.Items().GetSingle(context.Background(), req)
	return err

}

func updateItem(t *testing.T, tableSlug string, payload map[string]any) error {
	data, err := helper.ConvertMapToStruct(payload)
	if err != nil {
		return err
	}

	_, err = strg.Items().Update(
		context.Background(),
		&nb.CommonMessage{
			ProjectId: projectId,
			TableSlug: tableSlug,
			Data:      data,
		},
	)
	return err
}

func deleteMany(t *testing.T, tableSlug string, ids []string) error {
	dataMap := map[string]any{
		"from_auth_service": false,
		"ids":               ids,
	}

	data, err := helper.ConvertMapToStruct(dataMap)
	if err != nil {
		return err
	}

	_, err = strg.Items().DeleteMany(context.Background(),
		&nb.CommonMessage{
			ProjectId: projectId,
			TableSlug: tableSlug,
			Data:      data,
		},
	)
	return err
}

func deleteSingle(t *testing.T, tableSlug, id string) error {
	dataMap := map[string]any{
		"from_auth_service": false,
		"id":                id,
	}

	data, err := helper.ConvertMapToStruct(dataMap)
	if err != nil {
		return err
	}

	_, err = strg.Items().Delete(context.Background(),
		&nb.CommonMessage{
			ProjectId: projectId,
			TableSlug: tableSlug,
			Data:      data,
		})
	return err
}

func upsertMany(t *testing.T, tableSlug string, fieldSlug string, fields []string, objects []map[string]any) error {
	dataMap := map[string]any{
		"field_slug": fieldSlug,
		"fields":     fields,
		"objects":    objects,
	}

	data, err := helper.ConvertMapToStruct(dataMap)
	if err != nil {
		return err
	}

	err = strg.Items().UpsertMany(context.Background(),
		&nb.CommonMessage{
			ProjectId: projectId,
			TableSlug: tableSlug,
			Data:      data,
		},
	)
	return err
}

func deleteTable(t *testing.T, id string) error {
	err = strg.Table().Delete(context.Background(), &nb.TablePrimaryKey{Id: id, ProjectId: projectId})
	return err
}

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
			"is_visible": true,
			"unique":     false,
			"automatic":  false,
			"required":   true,
		},
	}
)

func TestItemsFlow(t *testing.T) {

	var mainTableId string
	var relatedTableId string
	var relatedId string
	var mainId string
	var existingId string
	var newId string
	var upsertNewId string

	// 1) Create main table
	t.Run("1 - Create main table", func(t *testing.T) {
		mainTableId, err = createTable(t, table1Slug, table1Label)
		assert.NoError(t, err)
	})

	// 2) Create fields for main table
	t.Run("2 - Create fields for main table", func(t *testing.T) {
		err = createFields(t, mainTableId, table1Fields)
		assert.NoError(t, err)
	})

	// 3) Create related table
	t.Run("3 - Create related table", func(t *testing.T) {
		relatedTableId, err = createTable(t, table2Slug, table2Label)
		assert.NoError(t, err)
	})

	// 4) Create fields for related table
	t.Run("4 - Create fields for related table", func(t *testing.T) {
		err = createFields(t, relatedTableId, table2Fields)
		assert.NoError(t, err)
	})

	// 5) Create relation for main table
	t.Run("5 - Create relation for main table", func(t *testing.T) {
		err = createRelation(t, table1Slug, table2Slug)
		assert.NoError(t, err)
	})

	// 6) Create related item (will be referenced via author_id)
	t.Run("6 - Create related item", func(t *testing.T) {
		relatedId, err = createItem(t, table2Slug, map[string]any{
			"from_auth_service": false,
			"name":              fakeData.Person().Name(),
		})
		assert.NoError(t, err)
	})

	// 7) Create main item with relation to relatedId
	t.Run("7 - Create main item with relation", func(t *testing.T) {
		mainId, err = createItem(t, table1Slug, map[string]any{
			"from_auth_service": false,
			"first_name":        fakeData.Person().FirstName(),
			"last_name":         fakeData.Person().LastName(),
			"test_table_2_id":   relatedId,
		})
		assert.NoError(t, err)
	})

	// 8) Get single item TODO ERROR
	t.Run("8 - Get single item", func(t *testing.T) {
		err = getSingleItem(t, table1Slug, "")
		assert.NoError(t, err)
	})

	// 9) Update main item
	t.Run("9 - Update main item", func(t *testing.T) {
		err = updateItem(t, table1Slug, map[string]any{
			"from_auth_service": false,
			"guid":              mainId,
			"first_name":        fakeData.Person().FirstName(),
		})
		assert.NoError(t, err)
	})

	// 10) MultipleUpdate: update existing and add new
	t.Run("10 - MultipleUpdate (update existing and add new)", func(t *testing.T) {
		existingId = mainId
		newId = uuid.NewString()
		objects := []map[string]any{
			{
				"from_auth_service": false,
				"guid":              existingId,
				"first_name":        fakeData.Person().FirstName(),
				"last_name":         fakeData.Person().LastName(),
				"is_new":            false,
			},
			{
				"from_auth_service": false,
				"guid":              newId,
				"first_name":        fakeData.Person().FirstName(),
				"last_name":         fakeData.Person().LastName(),
				"is_new":            true,
			},
		}
		dataMap := map[string]any{"objects": objects}
		data, err := helper.ConvertMapToStruct(dataMap)
		assert.NoError(t, err)

		_, err = strg.Items().MultipleUpdate(context.Background(), &nb.CommonMessage{ProjectId: projectId, TableSlug: table1Slug, Data: data})
		assert.NoError(t, err)
	})

	// 11) UpsertMany
	t.Run("11 - UpsertMany", func(t *testing.T) {
		existingIdForUpsert := existingId
		upsertNewId = uuid.NewString()

		upsertObjects := []map[string]any{
			{
				"guid":       existingIdForUpsert,
				"first_name": fakeData.Person().FirstName(),
				"last_name":  fakeData.Person().LastName(),
			},
			{
				"guid":       upsertNewId,
				"first_name": fakeData.Person().FirstName(),
				"last_name":  fakeData.Person().LastName(),
			},
		}
		err = upsertMany(t, table1Slug, "guid", []string{"guid", "first_name", "last_name"}, upsertObjects)
		assert.NoError(t, err)
	})

	// Objec Builder tests
	{
		t.Run("2.1 - Get List V2", func(t *testing.T) {
			err = GetListV2Test(t, table1Slug)
			assert.NoError(t, err)
		})

		t.Run("2.2 - Get List Aggregations", func(t *testing.T) {
			err = GetListAggregationTest(t, table1Slug)
			assert.NoError(t, err)
		})

		t.Run("2.3 - Get Table details", func(t *testing.T) {
			err = GetTableDetailsTest(t, table1Slug)
			assert.NoError(t, err)
		})

	}

	// 12) DeleteMany (remove existing and the one created by MultipleUpdate)
	t.Run("12 - DeleteMany", func(t *testing.T) {
		err = deleteMany(t, table1Slug, []string{existingId, newId})
		assert.NoError(t, err)
	})

	// 13) Delete single (remove the one created by UpsertMany)
	t.Run("13 - Delete single (upsert new)", func(t *testing.T) {
		err = deleteSingle(t, table1Slug, upsertNewId)
		assert.NoError(t, err)
	})

	// 14) Delete single (delete from related table)
	t.Run("14 - Delete single (related table)", func(t *testing.T) {
		err = deleteSingle(t, table2Slug, relatedId)
		assert.NoError(t, err)
	})

	// 15) Delete tables
	t.Run("15 - Delete tables", func(t *testing.T) {
		err = deleteTable(t, mainTableId)
		assert.NoError(t, err)

		err = deleteTable(t, relatedTableId)
		assert.NoError(t, err)
	})
}
