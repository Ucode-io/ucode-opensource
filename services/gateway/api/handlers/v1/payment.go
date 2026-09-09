package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/customer"
	"github.com/stripe/stripe-go/v83/setupintent"
	"github.com/stripe/stripe-go/v83/webhook"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
)

type createPaymentIntentRequest struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// CreatePaymentIntent calls Stripe to create a real SetupIntent (uses configured Stripe key)
func (h *HandlerV1) CreatePaymentIntent(c *gin.Context) {
	var (
		req        createPaymentIntentRequest
		customerId string
	)

	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "Invalid JSON")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	stripe.Key = h.baseConf.StripeApiKey

	project, err := h.companyServices.Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId.(string)})
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	customerId = project.CustomerId

	if project.CustomerId == "" {
		params := &stripe.CustomerParams{}
		if req.Email != "" {
			params.Email = stripe.String(req.Email)
		}
		if req.Phone != "" {
			params.Phone = stripe.String(req.Phone)
		}

		customer, err := customer.New(params)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		customerId = customer.ID

		_, err = h.companyServices.Project().AttachCustomer(ctx, &pb.AttachCustomerRequest{
			ProjectId:  projectId.(string),
			CustomerId: customerId,
		})
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	}

	intentParams := &stripe.SetupIntentParams{
		Customer:           stripe.String(customerId),
		PaymentMethodTypes: []*string{stripe.String("card")},
	}

	setI, err := setupintent.New(intentParams)
	if err != nil {
		fmt.Println("setupintent err", err)
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, setI)
}

func (h *HandlerV1) StripeWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "body read is failed. "+err.Error())
		return
	}

	event := stripe.Event{}

	if err := json.Unmarshal(payload, &event); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "json unmarshal is failed. "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	fmt.Println("payload", string(payload))

	endpointSecret := h.baseConf.StripeWebhookSecret
	signatureHeader := c.GetHeader("Stripe-Signature")
	fmt.Println("signatureHeader", signatureHeader)
	event, err = webhook.ConstructEventWithOptions(payload, signatureHeader, endpointSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	fmt.Println("Received event: ", string(event.Data.Raw))

	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			h.log.Error("Error parsing webhook JSON: ", logger.Error(err))
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		log.Printf("Successful payment for %d.", paymentIntent.Amount)
	case "payment_method.attached":
		var paymentMethod stripe.PaymentMethod
		err := json.Unmarshal(event.Data.Raw, &paymentMethod)
		if err != nil {
			h.log.Error("Error parsing webhook JSON: ", logger.Error(err))
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
		h.handlePaymentMethodAttached(ctx, paymentMethod)
	default:
		fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
	}

	h.HandleResponse(c, status_http.Created, status_http.OK)
}

func (h *HandlerV1) handlePaymentMethodAttached(ctx context.Context, paymentMethod stripe.PaymentMethod) {

	_, err := h.companyServices.Billing().CreateCard(ctx, &pb.CreateProjectCardRequest{
		Pan: fmt.Sprintf("****%s", paymentMethod.Card.Last4),
		Expire: func(month, year int64) string {
			return fmt.Sprintf("%02d/%02d", month, year%100)
		}(paymentMethod.Card.ExpMonth, paymentMethod.Card.ExpYear),
		ProjectId:  paymentMethod.Customer.ID,
		Type:       strings.ToUpper(string(paymentMethod.Card.Brand)),
		ExternalId: paymentMethod.ID,
		Verify:     true,
	})
	if err != nil {
		h.log.Error("Error while creating card: ", logger.Error(err))
	}

}
