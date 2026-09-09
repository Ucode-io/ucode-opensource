package handlers

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/eimzo"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetChallenge(c *gin.Context) {
	resp, err := eimzo.GetChallenge(h.cfg)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

func (h *Handler) VerifyUser(c *gin.Context) {
	var request pb.ExtractUserFromPKCS7Request

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := eimzo.ExtractUserFromPKCS7(h.cfg, request.Pkcs7, c.ClientIP())
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}
