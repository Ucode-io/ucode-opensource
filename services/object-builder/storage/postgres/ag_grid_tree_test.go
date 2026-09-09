package postgres_test

//
//import (
//	"context"
//	"testing"
//	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
//	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
//
//	"github.com/stretchr/testify/assert"
//)
//
//func TestAgGridTree(t *testing.T) {
//	treeRequest := map[string]any{
//		"startRow": 0,
//		"endRow":   100, 
//		"rowGroupCols": []map[string]any{
//			{
//				"id":          "country",
//				"displayName": "Country",
//				"field":       "country",
//			},
//		},
//		"valueCols": []map[string]any{
//			{
//				"id":          "gold",
//				"aggFunc":     "sum",
//				"displayName": "Gold",
//				"field":       "gold",
//			},
//			{
//				"id":          "silver",
//				"aggFunc":     "sum",
//				"displayName": "Silver",
//				"field":       "silver",
//			},
//			{
//				"id":          "bronze",
//				"aggFunc":     "sum",
//				"displayName": "Bronze",
//				"field":       "bronze",
//			},
//		},
//		"groupKeys":   []string{},
//		"filterModel": map[string]any{},
//		"sortModel":   []map[string]any{},
//		"pivotMode":   false,
//	}
//	resp, err := helper.ConvertMapToStruct(treeRequest)
//	assert.NoError(t, err)
//	request := &nb.CommonMessage{
//		TableSlug: "olympic",
//		ProjectId: "f0259839-c2fc-44e8-af90-1a6aa7ba43f7",
//		Data:      resp,
//	}
//
//	_, err = strg.ObjectBuilder().AgGridTree(context.Background(), request)
//	assert.NoError(t, err)
//}
