package v2

import (
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

type FilterOperator string

const (
	FilterEq        FilterOperator = "eq"
	FilterNeq       FilterOperator = "neq"
	FilterGt        FilterOperator = "gt"
	FilterGte       FilterOperator = "gte"
	FilterLt        FilterOperator = "lt"
	FilterLte       FilterOperator = "lte"
	FilterLike      FilterOperator = "like"
	FilterIlike     FilterOperator = "ilike"
	FilterNotLike   FilterOperator = "not_like"
	FilterIn        FilterOperator = "in"
	FilterIsNull    FilterOperator = "is_null"
	FilterIsNotNull FilterOperator = "is_not_null"
)

type FilterItem struct {
	Column   string         `json:"column"`
	Operator FilterOperator `json:"operator"`
	Value    any            `json:"value,omitempty"`
}

type GetListWithFiltersRequest struct {
	Filters []FilterItem `json:"filters"`
	Logic   string       `json:"logic"` // "AND" or "OR", defaults to "AND"
	Columns []string     `json:"columns,omitempty"`
	Limit   int          `json:"limit,omitempty"`
	Offset  int          `json:"offset,omitempty"`
}

func buildFilterWhereClause(filters []FilterItem, logic string) (string, error) {
	if logic == "" {
		logic = "AND"
	}
	logic = strings.ToUpper(logic)
	if logic != "AND" && logic != "OR" {
		return "", fmt.Errorf("logic must be AND or OR, got: %s", logic)
	}

	parts := make([]string, 0, len(filters))

	for _, f := range filters {
		if f.Column == "" {
			return "", fmt.Errorf("filter column cannot be empty")
		}
		col := fmt.Sprintf(`"%s"`, f.Column)

		var clause string
		switch f.Operator {
		case FilterIsNull:
			clause = fmt.Sprintf("%s IS NULL", col)

		case FilterIsNotNull:
			clause = fmt.Sprintf("%s IS NOT NULL", col)

		case FilterIn:
			vals, ok := toStringSlice(f.Value)
			if !ok || len(vals) == 0 {
				return "", fmt.Errorf("operator 'in' requires a non-empty array value for column %s", f.Column)
			}
			quoted := make([]string, len(vals))
			for i, v := range vals {
				quoted[i] = fmt.Sprintf("'%s'", escapeSingleQuote(v))
			}
			clause = fmt.Sprintf("%s IN (%s)", col, strings.Join(quoted, ", "))

		case FilterEq:
			clause = fmt.Sprintf("%s = %s", col, formatValue(f.Value))
		case FilterNeq:
			clause = fmt.Sprintf("%s != %s", col, formatValue(f.Value))
		case FilterGt:
			clause = fmt.Sprintf("%s > %s", col, formatValue(f.Value))
		case FilterGte:
			clause = fmt.Sprintf("%s >= %s", col, formatValue(f.Value))
		case FilterLt:
			clause = fmt.Sprintf("%s < %s", col, formatValue(f.Value))
		case FilterLte:
			clause = fmt.Sprintf("%s <= %s", col, formatValue(f.Value))
		case FilterLike:
			clause = fmt.Sprintf("%s LIKE '%s'", col, escapeSingleQuote(wrapWildcard(fmt.Sprintf("%v", f.Value))))
		case FilterIlike:
			clause = fmt.Sprintf("%s ILIKE '%s'", col, escapeSingleQuote(wrapWildcard(fmt.Sprintf("%v", f.Value))))
		case FilterNotLike:
			clause = fmt.Sprintf("%s NOT LIKE '%s'", col, escapeSingleQuote(wrapWildcard(fmt.Sprintf("%v", f.Value))))

		default:
			return "", fmt.Errorf("unsupported operator: %s", f.Operator)
		}

		parts = append(parts, clause)
	}

	return strings.Join(parts, " "+logic+" "), nil
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		// JSON numbers decode as float64
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("'%s'", escapeSingleQuote(fmt.Sprintf("%v", val)))
	}
}

func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// wrapWildcard adds % around the value if the user didn't include any wildcards,
// so "si" becomes "%si%" (contains search).
func wrapWildcard(s string) string {
	if strings.Contains(s, "%") {
		return s
	}
	return "%" + s + "%"
}

func toStringSlice(v any) ([]string, bool) {
	switch val := v.(type) {
	case []any:
		result := make([]string, len(val))
		for i, item := range val {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result, true
	case []string:
		return val, true
	}
	return nil, false
}

// GetListWithFilters godoc
// @Security ApiKeyAuth
// @ID get_list_with_filters
// @Router /v2/items/{collection}/filter [POST]
// @Summary Get list with filters
// @Description Queries a collection with structured filters, building a SQL WHERE clause automatically
// @Tags Items
// @Accept json
// @Produce json
// @Param collection path string true "collection (table slug)"
// @Param body body GetListWithFiltersRequest true "Filter request"
// @Success 200 {object} status_http.Response "OK"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetListWithFilters(c *gin.Context) {
	var req GetListWithFiltersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	tableSlug := c.Param("collection")

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	whereClause, err := buildFilterWhereClause(req.Filters, req.Logic)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	data := map[string]any{
		"operation": "SELECT",
		"table":     tableSlug,
	}
	if len(req.Columns) > 0 {
		data["columns"] = req.Columns
	}
	if whereClause != "" {
		data["where"] = whereClause
	}
	if req.Limit > 0 {
		data["limit"] = req.Limit
	}
	if req.Offset > 0 {
		data["offset"] = req.Offset
	}

	structData, err := helper.ConvertMapToStruct(data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	fmt.Println("Struct data->", structData)

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp := &obs.CommonMessage{}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetListAggregation(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		pgResp, err := services.GoObjectBuilderService().ObjectBuilder().GetListAggregation(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		if err = helper.MarshalToStruct(pgResp, &resp); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	h.HandleResponse(c, status_http.OK, resp)
}
