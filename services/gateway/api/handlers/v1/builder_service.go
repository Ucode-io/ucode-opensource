package v1

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
)

// getBuilderServices resolves the object-builder service and resource
// environment for the project and environment carried on the request.
//
// Every handler that talks to a project's own database needs this, so it does
// not belong to any one feature.
func (h *HandlerV1) getBuilderServices(c *gin.Context) (services.ServiceManagerI, string, error) {
	service, resource, err := h.resolveBuilderService(c, c.Request.Context())
	if err != nil {
		return nil, "", err
	}
	return service, resource.GetResourceEnvironmentId(), nil
}

func (h *HandlerV1) resolveBuilderService(c *gin.Context, ctx context.Context) (services.ServiceManagerI, *pb.ServiceResourceModel, error) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return nil, nil, config.ErrProjectIdValid
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return nil, nil, config.ErrEnvironmentIdValid
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return nil, nil, err
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return nil, nil, config.ErrProjectIdValid
	}

	service, err := h.GetProjectSrvc(ctx, projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return nil, nil, err
	}

	return service, resource, nil
}
