package v1

import (
	"context"
	"encoding/json"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
)

func (h *HandlerV1) Proxy(c *gin.Context) {
	h.HandleResponse(c, status_http.OK, "PROXY response")
}

func (h *HandlerV1) CompanyRedirectGetList(data helper.MatchingData, comp services.CompanyServiceI) (*pb.GetListRedirectUrlRes, error) {
	var (
		key = "redirect-" + data.ProjectId + data.EnvId
		res = &pb.GetListRedirectUrlRes{}
		err error
	)

	var redirectWaitKey = config.CACHE_WAIT + "-redirect"
	_, redirectOk := h.cache.Get(redirectWaitKey)
	if redirectOk {
		ctx, cancel := context.WithTimeout(context.Background(), config.REDIS_WAIT_TIMEOUT)
		defer cancel()

		for {
			redirectBody, ok := h.cache.Get(key)
			if ok {
				err = json.Unmarshal(redirectBody, &res)
				if err != nil {
					return nil, err
				}
			}

			if len(res.RedirectUrls) > 0 {
				return res, nil
			}

			if ctx.Err() == context.DeadlineExceeded {
				break
			}

			time.Sleep(config.REDIS_SLEEP)
		}
	} else {
		h.cache.Add(redirectWaitKey, []byte(redirectWaitKey), config.REDIS_KEY_TIMEOUT)
	}

	res, err = comp.Redirect().GetList(context.Background(), &pb.GetListRedirectUrlReq{
		ProjectId: data.ProjectId,
		EnvId:     data.EnvId,
		Offset:    0,
		Limit:     100,
	})
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}

	h.cache.Add(key, body, config.REDIS_TIMEOUT)

	return res, nil
}
