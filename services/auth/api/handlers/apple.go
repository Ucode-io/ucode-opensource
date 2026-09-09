package handlers

import (
	"context"
	"errors"

	status "github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateAppleIdSettings godoc
// @ID createAppleIdSettings
// @Router /v2/apple-id-settings [POST]
// @Summary CreateAppleIdSettings
// @Description CreateAppleIdSettings
// @Tags AppleId
// @Accept json
// @Produce json
// @Param X-API-KEY header string false "X-API-KEY"
// @Param registerBody body auth_service.AppleIdSettings true "register_body"
// @Success 201 {object} http.Response{data=auth_service.AppleIdSettings} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) CreateAppleIdSettings(c *gin.Context) {
	var body *pb.AppleIdSettings

	err := c.ShouldBindJSON(&body)
	if err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	uuid, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	resp, err := h.services.AppleIdService().CreateAppleIdSettings(
		c.Request.Context(), &pb.AppleIdSettings{
			Id:        uuid.String(),
			ProjectId: body.ProjectId,
			ClientId:  body.ClientId,
			TeamId:    body.TeamId,
			KeyId:     body.KeyId,
			Secret:    body.Secret,
		},
	)
	if err != nil {
		h.log.Error("---> error in create apple id settings", logger.Error(err))
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.Created, resp)
}

// UpdateAppleIdSettings godoc
// @ID updateAppleIdSettings
// @Router /v2/apple-id-settings [PUT]
// @Summary UpdateAppleIdSettings
// @Description UpdateAppleIdSettings
// @Tags AppleId
// @Accept json
// @Produce json
// @Param registerBody body auth_service.AppleIdSettings true "register_body"
// @Success 200 {object} http.Response{data=auth_service.AppleIdSettings} "Apple Config data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UpdateAppleIdSettings(c *gin.Context) {
	var body *pb.AppleIdSettings

	if err := c.ShouldBindJSON(&body); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	resp, err := h.services.AppleIdService().UpdateAppleIdSettings(
		c.Request.Context(), &pb.AppleIdSettings{
			Id:        body.Id,
			ProjectId: body.ProjectId,
			ClientId:  body.ClientId,
			TeamId:    body.TeamId,
			KeyId:     body.KeyId,
			Secret:    body.Secret,
		},
	)
	if err != nil {
		h.log.Error("---> error in update apple Id settings", logger.Error(err))
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, resp)
}

// GetListAppleIdSettings godoc
// @ID getListAppleIdSettings
// @Router /v2/apple-id-settings [GET]
// @Summary GetListAppleIdSettings
// @Description GetListAppleIdSettings
// @Tags AppleId
// @Accept json
// @Produce json
// @Param project_id query string true "project_id"
// @Success 200 {object} http.Response{data=auth_service.GetListAppleIdSettingsResponse} "Apple Config data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetAppleIdSettings(c *gin.Context) {
	resp, err := h.services.AppleIdService().GetListAppleIdSettings(
		c.Request.Context(), &pb.GetListAppleIdSettingsRequest{
			ProjectId: c.Query("project_id"),
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, resp)
}

// DeleteAppleIdSettings godoc
// @ID deleteAppleIdSettings
// @Router /v2/apple-id-settings/{id} [DELETE]
// @Summary DeleteAppleIdSettings
// @Description DeleteAppleIdSettings
// @Tags AppleId
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteAppleIdSettings(c *gin.Context) {
	resp, err := h.services.AppleIdService().DeleteAppleIdSettings(
		c.Request.Context(), &pb.AppleIdSettingsPrimaryKey{
			Id: c.Param("id"),
		},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.NoContent, resp)
}

func (h *Handler) GetAppleConfig(projectId string, ctx context.Context) (*models.AppleConfig, error) {
	resp, err := h.services.AppleIdService().GetListAppleIdSettings(
		ctx, &pb.GetListAppleIdSettingsRequest{
			ProjectId: projectId,
		},
	)
	if err != nil {
		return nil, err
	}

	if len(resp.Items) < 1 {
		return nil, errors.New("project hasn't apple configs")
	}

	return &models.AppleConfig{
		TeamId:    resp.Items[0].TeamId,
		ClientId:  resp.Items[0].ClientId,
		KeyId:     resp.Items[0].KeyId,
		SecretKey: resp.Items[0].Secret,
	}, nil

}
