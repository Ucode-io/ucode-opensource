package apilimits

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/gin-gonic/gin"
)

// BillingLimitMiddleware aborts requests when a project's monthly API call
// limit flag is set to "0" in Redis by BillingLimitWorker.
func (t *Tracker) BillingLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.GetString("project_id")
		if projectID == "" || config.RateLimitSkipFiles[c.Param("collection")] {
			c.Next()
			return
		}

		limitKey := fmt.Sprintf(config.KeyBillingApiLimit, projectID)
		if val, err := t.rdb.Get(context.Background(), limitKey).Result(); err == nil && val == "0" {
			c.AbortWithStatusJSON(http.StatusPaymentRequired, status_http.Response{
				Status:      status_http.PaymentRequired.Status,
				Description: "Monthly API call limit exceeded. Please upgrade your plan.",
				Data:        models.PaymentApiCallLimit,
			})
			return
		}

		c.Next()
	}
}

// ApiCallCountMiddleware increments the per-project L1 counter on every request.
// Counts are flushed to Redis by Tracker on its flush interval.
func (t *Tracker) ApiCallCountMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if projectID := c.GetString("project_id"); projectID != "" {
			counter, _ := t.l1Cache.LoadOrStore(projectID, &atomic.Int64{})
			counter.(*atomic.Int64).Add(1)
		}
		c.Next()
	}
}
