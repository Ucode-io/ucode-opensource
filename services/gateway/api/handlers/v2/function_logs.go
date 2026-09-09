package v2

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func (h *HandlerV2) GetListFunctionLogs(c *gin.Context) {

	var (
		projectId     any
		environmentId any
		ok            bool

		tableSlug     = cast.ToString(c.Query("table"))
		requestMethod = cast.ToString(c.Query("method"))
		actionType    = cast.ToString(c.Query("action_type"))
		status        = cast.ToString(c.Query("status"))
		fromDate      = cast.ToString(c.Query("from_date"))
		toDate        = cast.ToString(c.Query("to_date"))

		limit      = cast.ToInt64(c.Query("limit"))
		offset     = cast.ToInt64(c.Query("offset"))
		functionId = c.Query("function_id")
		search     = c.Query("search")
	)

	if limit == 0 {
		limit = 100
	}

	projectId, ok = c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok = c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     cast.ToString(projectId),
			EnvironmentId: cast.ToString(environmentId),
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

	var request = nb.GetFunctionLogsReq{
		ProjectId:     resource.ResourceEnvironmentId,
		FunctionId:    functionId,
		TableSlug:     tableSlug,
		RequestMethod: requestMethod,
		ActionType:    actionType,
		Status:        status,
		FromDate:      fromDate,
		ToDate:        toDate,
		Search:        search,
		Limit:         limit,
		Offset:        offset,
	}

	if resource.ResourceType == pb.ResourceType_POSTGRESQL {
		response, err := services.GoObjectBuilderService().VersionHistory().GetFunctionLogs(c, &request)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, response)
		return
	}

	h.HandleResponse(c, status_http.BadRequest, "does not implemented")
	return
}
