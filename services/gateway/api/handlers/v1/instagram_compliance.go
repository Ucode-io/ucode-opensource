package v1

import (
	"net/http"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
)

func (h *HandlerV1) InstagramDeauthorize(c *gin.Context) {
	value := strings.TrimSpace(c.PostForm("signed_request"))
	secrets := append([]string{h.baseConf.InstagramClientSecret}, h.baseConf.InstagramLegacyClientSecrets...)
	payload, err := parseMetaSignedRequest(value, secrets...)
	if err != nil {
		h.log.Warn("instagram deauthorize: signed request rejected", logger.Error(err))
		c.JSON(http.StatusForbidden, gin.H{"success": false})
		return
	}

	h.log.Info("instagram deauthorize: accepted", logger.String("instagram_user_id", payload.UserID))
	c.JSON(http.StatusOK, gin.H{"success": true})
}
