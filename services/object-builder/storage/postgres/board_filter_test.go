package postgres

import (
	"strings"
	"testing"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestParseBoardParamsKeepsRequestDataShape(t *testing.T) {
	data, err := structpb.NewStruct(map[string]any{
		"group_by": map[string]any{"field": "status"},
		"fields":   []any{"guid", "status"},
		"tables":   []any{map[string]any{"table_slug": "branches", "object_id": "branch-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	params, paramsMap, err := parseBoardParams(&nb.CommonMessage{Data: data})
	if err != nil {
		t.Fatal(err)
	}

	if params.GroupBy.Field != "status" {
		t.Fatalf("expected group_by field status, got %q", params.GroupBy.Field)
	}
	if len(params.Fields) != 2 {
		t.Fatalf("expected two fields, got %#v", params.Fields)
	}
	if _, ok := paramsMap["tables"].([]any); !ok {
		t.Fatalf("expected tables to stay as array in params map, got %#v", paramsMap["tables"])
	}
}

func TestBuildBoardWhereClauseFromParamsSkipsServiceFields(t *testing.T) {
	params := map[string]any{
		"tables":                    []any{map[string]any{"table_slug": "branches", "object_id": "branch-1"}},
		"role_id_from_token":        "role-1",
		"user_id_from_token":        "user-1",
		"client_type_id_from_token": "client-type-1",
		"fields":                    []any{"guid", "status"},
		"group_by":                  map[string]any{"field": "status"},
		"status":                    []any{"new"},
		"unknown_filter":            []any{"ignored"},
	}
	tableColumns := map[string]bool{
		"deleted_at": true,
		"guid":       true,
		"status":     true,
	}

	whereClause, args := buildBoardWhereClauseFromParams(params, tableColumns, "", nil, 3)

	if strings.Contains(whereClause, `"tables"`) {
		t.Fatalf("service field tables must not be used as SQL column: %s", whereClause)
	}
	if strings.Contains(whereClause, "unknown_filter") {
		t.Fatalf("unknown request field must not be used as SQL column: %s", whereClause)
	}
	if !strings.Contains(whereClause, `a."status"::TEXT = ANY($3::TEXT[])`) {
		t.Fatalf("expected status array filter, got: %s", whereClause)
	}
	if len(args) != 1 {
		t.Fatalf("expected one SQL arg, got %d: %#v", len(args), args)
	}
}

func TestBuildBoardWhereClauseFromParamsIncludesAutoFilter(t *testing.T) {
	params := map[string]any{
		"auto_filter": map[string]any{
			"branches_id": "branch-1",
			"tables":      "ignored",
		},
	}
	tableColumns := map[string]bool{
		"deleted_at":  true,
		"branches_id": true,
	}

	whereClause, args := buildBoardWhereClauseFromParams(params, tableColumns, "", nil, 1)

	if !strings.Contains(whereClause, `a."branches_id"::TEXT = $1::TEXT`) {
		t.Fatalf("expected branches_id auto filter, got: %s", whereClause)
	}
	if strings.Contains(whereClause, `"tables"`) {
		t.Fatalf("invalid auto filter field must not be used as SQL column: %s", whereClause)
	}
	if len(args) != 1 || args[0] != "branch-1" {
		t.Fatalf("expected branch auto filter arg, got %#v", args)
	}
}

func TestBuildBoardWhereClauseFromParamsSearchUsesOnlyTableColumns(t *testing.T) {
	tableColumns := map[string]bool{
		"deleted_at":   true,
		"order_number": true,
	}

	whereClause, args := buildBoardWhereClauseFromParams(map[string]any{}, tableColumns, "A-1", []string{"order_number", "tables"}, 1)

	if !strings.Contains(whereClause, `a."order_number"::TEXT ~* $1::TEXT`) {
		t.Fatalf("expected search filter for real column, got: %s", whereClause)
	}
	if strings.Contains(whereClause, `"tables"`) {
		t.Fatalf("search must ignore non-table fields: %s", whereClause)
	}
	if len(args) != 1 || args[0] != "A-1" {
		t.Fatalf("expected search arg, got %#v", args)
	}
}

func TestBuildBoardCountQueryUsesProvidedWhereClause(t *testing.T) {
	whereClause := `a.deleted_at IS NULL AND a."branches_id"::TEXT = $1::TEXT`
	query := buildBoardCountQuery("orders", whereClause)

	if !strings.Contains(query, whereClause) {
		t.Fatalf("count query must reuse board where clause, got: %s", query)
	}
	if !strings.Contains(query, `FROM "orders" a`) {
		t.Fatalf("count query must quote table slug, got: %s", query)
	}
}

func TestBoardWhereClauseCanStartAtDifferentParamIndexes(t *testing.T) {
	params := map[string]any{
		"auto_filter": map[string]any{
			"branches_id": "branch-1",
		},
	}
	tableColumns := map[string]bool{
		"deleted_at":  true,
		"branches_id": true,
	}

	dataWhereClause, _ := buildBoardWhereClauseFromParams(params, tableColumns, "", nil, 3)
	countWhereClause, _ := buildBoardWhereClauseFromParams(params, tableColumns, "", nil, 1)

	if !strings.Contains(dataWhereClause, `$3::TEXT`) {
		t.Fatalf("data where clause must reserve $1/$2 for offset/limit, got: %s", dataWhereClause)
	}
	if !strings.Contains(countWhereClause, `$1::TEXT`) {
		t.Fatalf("count where clause must start from first parameter, got: %s", countWhereClause)
	}
	if strings.Contains(countWhereClause, `$3::TEXT`) {
		t.Fatalf("count where clause must not reuse data query placeholders, got: %s", countWhereClause)
	}
}

func TestBuildBoardDataQueryCastsPaginationParams(t *testing.T) {
	query := buildBoardDataQuery("orders", `a.guid, a.status`, `a.deleted_at IS NULL`, "created_at")

	if !strings.Contains(query, `OFFSET $1::INT`) {
		t.Fatalf("board data query must cast offset param, got: %s", query)
	}
	if !strings.Contains(query, `LIMIT $2::INT`) {
		t.Fatalf("board data query must cast limit param, got: %s", query)
	}
}
