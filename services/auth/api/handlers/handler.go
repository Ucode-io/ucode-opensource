package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	status_http "github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"

	"github.com/google/uuid"
	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	services    client.ServiceManagerI
	serviceNode service.ServiceNodesI
}

func NewHandler(cfg config.BaseConfig, log logger.LoggerI, svcs client.ServiceManagerI, serviceNode service.ServiceNodesI) Handler {
	return Handler{
		cfg:         cfg,
		log:         log,
		services:    svcs,
		serviceNode: serviceNode,
	}
}

func (h *Handler) GetProjectSrvc(c *gin.Context, projectId string, nodeType string) (client.SharedServiceManagerI, error) {
	if nodeType == config.ENTER_PRICE_TYPE {
		srvc, err := h.serviceNode.Get(projectId)
		if err != nil {
			return nil, err
		}

		return srvc, nil
	} else {
		srvc, err := h.serviceNode.Get(h.cfg.UcodeNamespace)
		if err != nil {
			return nil, err
		}

		return srvc, nil
	}
}

func (h *Handler) handleResponse(c *gin.Context, status status_http.Status, data any) {
	switch code := status.Code; {
	case code < 300:
	case code < 400:
		h.log.Warn(
			"!!!Response--->",
			logger.Int("code", status.Code),
			logger.String("status", status.Status),
			logger.Any("description", status.Description),
			logger.Any("data", data),
		)
	default:
		h.log.Error(
			"!!!Response--->",
			logger.Int("code", status.Code),
			logger.String("status", status.Status),
			logger.Any("description", status.Description),
			logger.Any("data", data),
		)
	}

	c.JSON(status.Code, status_http.Response{
		Status:      status.Status,
		Description: status.Description,
		Data:        data,
	})
}

func (h *Handler) handleError(c *gin.Context, statusHttp status_http.Status, err error) {
	st, ok := status.FromError(err)
	if !ok {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.ErrWrong,
		})
	}

	if statusHttp.Status == status_http.BadRequest.Status {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.ErrInvalidJSON,
		})
	} else if st.Code() == codes.AlreadyExists {
		var data string
		if st.Message() == config.EmailConstraint {
			data = config.ErrEmailExists
		} else if st.Message() == config.PhoneConstraint {
			data = config.ErrPhoneExists
		} else {
			data = st.Message()
		}

		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        data,
		})
	} else if st.Code() == codes.InvalidArgument {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        st.Message(),
		})
	} else if st.Code() == codes.Unimplemented {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.ErrOutOfWork,
		})
	} else if statusHttp.Status == status_http.Unauthorized.Status {
		c.JSON(http.StatusUnauthorized, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.SessionExpired,
		})
	} else if statusHttp.Status == status_http.Forbidden.Status {
		c.JSON(http.StatusForbidden, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        config.SessionExpired,
		})
	} else if st.Code() == codes.ResourceExhausted {
		c.JSON(http.StatusPaymentRequired, status_http.Response{
			Status:      status_http.PaymentRequired.Status,
			Description: status_http.PaymentRequired.Description,
			Data: models.PaymentRequiredData{
				Type: "payment_required",
			},
		})
	} else if st.Code() == codes.FailedPrecondition {
		// Insufficient head-project balance for a paid action (e.g. a paid user seat).
		c.JSON(http.StatusPaymentRequired, status_http.Response{
			Status:      status_http.PaymentRequired.Status,
			Description: status_http.PaymentRequired.Description,
			Data: models.PaymentRequiredData{
				Type: "payment_required",
				Code: "insufficient_balance",
			},
		})
	} else if st.Err() != nil {
		c.JSON(http.StatusInternalServerError, status_http.Response{
			Status:      statusHttp.Status,
			Description: st.String(),
			Data:        strings.ToUpper(st.Message()[:1]) + st.Message()[1:],
		})
	}
}

func paymentRequiredFromLimit(msg string) models.PaymentRequiredData {
	data := models.PaymentRequiredData{
		Type: "payment_required",
		Code: service.LimitCodeUsers,
		Unit: "users",
	}
	if strings.HasPrefix(msg, service.LimitCodeBuilders+":") {
		data.Code = service.LimitCodeBuilders
		data.Unit = "builders"
	}
	return data
}

func (h *Handler) getOffsetParam(c *gin.Context) (offset int, err error) {
	offsetStr := c.DefaultQuery("offset", h.cfg.DefaultOffset)
	return strconv.Atoi(offsetStr)
}

func (h *Handler) getLimitParam(c *gin.Context) (offset int, err error) {
	limitStr := c.DefaultQuery("limit", h.cfg.DefaultLimit)
	return strconv.Atoi(limitStr)
}

func (h *Handler) versionHistory(req *models.CreateVersionHistoryRequest) error {
	var (
		current  = map[string]any{"data": req.Current}
		previous = map[string]any{"data": req.Previous}
		request  = map[string]any{"data": req.Request}
		response = map[string]any{"data": req.Response}
		user     = req.UserInfo
	)

	sharedServiceManager, err := h.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return err
	}

	if req.Current == nil {
		current["data"] = make(map[string]any)
	}
	if req.Previous == nil {
		previous["data"] = make(map[string]any)
	}
	if req.Request == nil {
		request["data"] = make(map[string]any)
	}
	if req.Response == nil {
		response["data"] = make(map[string]any)
	}

	if util.IsValidUUID(req.UserInfo) {
		info, err := h.services.UserService().GetUserByID(context.Background(), &auth_service.UserPrimaryKey{
			Id: req.UserInfo,
		})
		if err == nil {
			if info.Login != "" {
				user = info.Login
			} else {
				user = info.Phone
			}
		}
	}

	_, err = sharedServiceManager.VersionHistoryService().Create(
		context.Background(),
		&object_builder_service.CreateVersionHistoryRequest{
			Id:           uuid.NewString(),
			ProjectId:    req.ProjectId,
			ActionSource: req.ActionSource,
			ActionType:   req.ActionType,
			Previus:      fromMapToString(previous),
			Current:      fromMapToString(current),
			Date:         time.Now().Format("2006-01-02T15:04:05.000Z"),
			UserInfo:     user,
			Request:      fromMapToString(request),
			Response:     fromMapToString(response),
			ApiKey:       req.ApiKey,
			Type:         req.Type,
			TableSlug:    req.TableSlug,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func fromMapToString(req map[string]any) string {
	reqString, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	return string(reqString)
}
