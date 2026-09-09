package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/models"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *BillingService) GetVerifyCode(ctx context.Context, req *pb.GetVerifyCodeRequest) (*pb.GetVerifyCodeResponse, error) {
	dbSpan, _ := span.StartSpanFromContext(ctx, "grpc_billing.GetVerifyCodeRequest", req)
	defer dbSpan.Finish()

	payload := map[string]any{
		"id":     helper.GenerateRandomNumber(6),
		"method": config.PaymeCreateCard,
		"params": map[string]any{
			"card": map[string]any{
				"number": req.Pan,
				"expire": req.Expire,
			},
			"save": true,
		},
	}
	cardResp, err := sendRequest[models.CreateCardResponse](payload, s.config.PaymeApiUrl, s.config.PaymeXAuth)
	if err != nil {
		s.logger.Error("--GetVerifyCodeRequest--create card", l.Error(errors.New(config.ErrBadRequest)))
		return nil, status.Error(codes.Internal, config.ErrBadRequest)
	}
	if cardResp.Error != nil {
		s.logger.Error("--GetVerifyCodeRequest--create card response", l.Error(errors.New(cardResp.Error.Message)))
		return nil, status.Error(codes.Internal, cardResp.Error.Message)
	}

	payload = map[string]any{
		"id":     cardResp.Id,
		"method": config.PaymeGetVerifyCode,
		"params": map[string]any{
			"token": cardResp.Result.Card.PaymeToken,
		},
	}
	verifyResp, err := sendRequest[models.GetVerifyCodeResponse](payload, s.config.PaymeApiUrl, s.config.PaymeXAuth)
	if err != nil {
		s.logger.Error("--GetVerifyCodeRequest--get verify code", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	if verifyResp.Error != nil {
		s.logger.Error("--GetVerifyCodeRequest--get verify code response", l.Error(errors.New(verifyResp.Error.Message)))
		return nil, status.Error(codes.Internal, verifyResp.Error.Message)
	}

	resp, err := s.storage.Billing().CreateCard(
		ctx,
		&pb.CreateProjectCardRequest{
			Pan:        cardResp.Result.Card.Number,
			Expire:     cardResp.Result.Card.Expire,
			PaymeToken: cardResp.Result.Card.PaymeToken,
			ProjectId:  req.ProjectId,
			Type:       "UZCARD",
		},
	)
	if err != nil {
		s.logger.Error("--GetVerifyCode--storage create card", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetVerifyCodeResponse{
		Sent:          verifyResp.Result.Sent,
		Phone:         verifyResp.Result.Phone,
		Wait:          verifyResp.Result.Wait,
		ProjectCardId: resp.Id,
	}, nil
}

func (s *BillingService) Verify(ctx context.Context, req *pb.VerifyRequest) (*pb.ProjectCard, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.Verify", req)
	defer dbSpan.Finish()

	projectCard, err := s.storage.Billing().GetProjectCard(
		ctx,
		&pb.PrimaryKey{Id: req.ProjectCardId},
	)
	if err != nil {
		s.logger.Error("--Verify--get project card", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	payload := map[string]any{
		"id":     helper.GenerateRandomNumber(6),
		"method": config.PaymeVerify,
		"params": map[string]any{
			"token": projectCard.PaymeToken,
			"code":  req.Code,
		},
	}
	response, err := sendRequest[models.CreateCardResponse](payload, s.config.PaymeApiUrl, s.config.PaymeXAuth)
	if err != nil {
		s.logger.Error("--Verify--verify", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	if response.Error != nil {
		s.logger.Error("--Verify--verify response", l.Error(errors.New(response.Error.Message)))
		return nil, status.Error(codes.Internal, response.Error.Message)
	}

	resp, err := s.storage.Billing().UpdateProjectCard(
		ctx,
		&pb.ProjectCard{
			Id:         req.ProjectCardId,
			Pan:        response.Result.Card.Number,
			Expire:     response.Result.Card.Expire,
			PaymeToken: response.Result.Card.PaymeToken,
			Verify:     true,
			ProjectId:  req.ProjectId,
		},
	)
	if err != nil {
		s.logger.Error("--Verify--update project card", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) CreateCard(ctx context.Context, req *pb.CreateProjectCardRequest) (*pb.ProjectCard, error) {
	s.logger.Info("--CreateCard-- requested", l.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CreateCard", req)
	defer dbSpan.Finish()

	projectCard, err := s.storage.Billing().CreateCard(ctx, req)
	if err != nil {
		s.logger.Error("--CreateCard--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return projectCard, nil
}

func (s *BillingService) ListProjectCards(ctx context.Context, req *pb.ListRequest) (*pb.ListProjectCardsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ListProjectCards", req)
	defer dbSpan.Finish()

	s.logger.Info("--ListProjectCards-- requested")

	projectCards, err := s.storage.Billing().ListProjectCards(ctx, req)
	if err != nil {
		s.logger.Error("--ListProjectCards--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return projectCards, nil
}

func (s *BillingService) ReceiptPay(ctx context.Context, req *pb.ReceiptPayRequest) (*pb.ReceiptPayResponse, error) {
	s.logger.Info("--ReceiptPay-- requested", l.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ReceiptPay", req)
	defer dbSpan.Finish()

	var (
		paymentStatus string = config.PaymePaymentStatusCancelled
		comment       string
		organization  string
	)

	projectCard, err := s.storage.Billing().GetProjectCard(
		ctx, &pb.PrimaryKey{Id: req.ProjectCardId},
	)
	if err != nil {
		s.logger.Error("--ReceiptPay--get project card", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	project, err := s.storage.Project().GetById(ctx, req.ProjectId)
	if err != nil {
		s.logger.Error("--ReceiptPay--project get by id", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	switch projectCard.Type {
	case "UZCARD", "HUMO":
		orderId := helper.GenerateRandomNumber(6)
		payload := map[string]any{
			"id":     helper.GenerateRandomNumber(6),
			"method": config.ReceiptCreate,
			"params": map[string]any{
				"amount": req.Amount * 100,
				"account": map[string]any{
					"order_id": orderId,
				},
			},
		}

		xauth := fmt.Sprintf("%v:%v", s.config.PaymeXAuth, s.config.PaymeKey)
		receiptCreateResp, err := sendRequest[models.ReceiptCreateResponse](payload, s.config.PaymeApiUrl, xauth)
		if err != nil {
			s.logger.Error("--ReceiptPay--receipt create", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
		if receiptCreateResp.Error != nil {
			s.logger.Error("--ReceiptPay--receipt create response", l.Error(errors.New(receiptCreateResp.Error.Message)))
			return nil, status.Error(codes.Internal, receiptCreateResp.Error.Message)
		}

		transactionCreate, err := s.storage.Billing().CreateTransaction(ctx,
			&pb.CreateTransactionRequest{
				ProjectId:       req.ProjectId,
				CreatorId:       req.UserId,
				PaymentStatus:   config.PaymePaymentStatusPending,
				Amount:          req.Amount,
				TransactionType: config.TransactionTypeTopup,
				CreatorType:     config.PaymeCreatorType,
				CurrencyId:      config.PaymeCurrencyId,
				FareId:          project.FareId,
				OrderId:         int64(orderId),
				PaymentType:     config.PaymePaymentType,
			},
		)
		if err != nil {
			s.logger.Error("--ReceiptPay--transaction create", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		defer func() {
			_, err := s.storage.Billing().UpdateTransaction(ctx,
				&pb.Transaction{
					Id:            transactionCreate.Id,
					PaymentStatus: paymentStatus,
					Comment:       comment,
				},
			)
			if err != nil {
				s.logger.Error("--ReceiptPay--update transaction", l.Error(err))
			}
		}()

		payload = map[string]any{
			"id":     receiptCreateResp.ID,
			"method": config.ReceiptPay,
			"params": map[string]any{
				"id":    receiptCreateResp.Result.Receipt.ID,
				"token": projectCard.PaymeToken,
			},
		}

		receiptPayResp, err := sendRequest[models.ReceiptCreateResponse](payload, s.config.PaymeApiUrl, xauth)
		if err != nil {
			s.logger.Error("--ReceiptPay--receipt pay", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
		if receiptPayResp.Error != nil {
			s.logger.Error("--ReceiptPay--receipt pay response", l.Error(errors.New(receiptPayResp.Error.Message)))
			comment = config.ChequeState[receiptPayResp.Result.Receipt.State]
			return nil, status.Error(codes.Internal, receiptPayResp.Error.Message)
		}

		organization = receiptPayResp.Result.Receipt.Merchant.Organization
		state := receiptPayResp.Result.Receipt.State
		comment = config.ChequeState[state]
		if state == config.PaymeStatusPaid {
			paymentStatus = config.PaymePaymentStatusAccepted
		}
	case "VISA":

		s.logger.Info("--ReceiptPay--visa cannot process payment with visa card", l.Any("req", req))
		return nil, status.Error(codes.ResourceExhausted, "cannot process payment with visa card")

		//rate, err := helper.FetchCurrencyRate(ctx, config.CURRENCY_USD, time.Now().Format(time.DateOnly))
		//if err != nil {
		//	return nil, fmt.Errorf("failed to fetch currency rate: %w", err)
		//}
		//
		//paymentIntent, err := cron.CreatePaymentIntent(s.config.StripeApiKey, int64(req.Amount*100), project.CustomerId, projectCard.ExternalId)
		//if err != nil {
		//	s.logger.Error("--ReceiptPay--create payment intent", l.Error(err))
		//	return nil, status.Error(codes.Internal, err.Error())
		//}
		//
		//_, err = s.storage.Billing().CreateTransaction(ctx,
		//	&pb.CreateTransactionRequest{
		//		ProjectId:       req.ProjectId,
		//		CreatorId:       req.UserId,
		//		PaymentStatus:   string(paymentIntent.Status),
		//		Amount:          req.Amount,
		//		TransactionType: config.TransactionTypeTopup,
		//		CreatorType:     config.PaymeCreatorType,
		//		CurrencyId:      config.USD_CURRENCY_ID,
		//		FareId:          project.FareId,
		//		OrderId:         0,
		//		PaymentType:     config.STRIPE_PAYMENT_TYPE,
		//		Rate:            rate,
		//		CardId:          projectCard.Id,
		//		ExternalId:      paymentIntent.ID,
		//	},
		//)
		//if err != nil {
		//	s.logger.Error("--ReceiptPay--transaction create", l.Error(err))
		//	return nil, status.Error(codes.Internal, err.Error())
		//}
		//
		//amountWithRate := req.Amount * rate
		//err = s.storage.Billing().AddBalanceToProject(ctx, req.ProjectId, amountWithRate)
		//if err != nil {
		//	s.logger.Error("--ReceiptPay-- AddBalanceToProject", l.Error(err))
		//	return nil, status.Error(codes.Internal, err.Error())
		//}
	}

	go func() {
		err = s.storage.Billing().ProcessUcodeRenewals(ctx)
		if err != nil {
			s.logger.Error("--ReceiptPay--process ucode renewals", l.Error(err))
		}
		err = s.storage.Billing().ProcessUgenRenewals(ctx)
		if err != nil {
			s.logger.Error("--ReceiptPay--process ugen renewals", l.Error(err))
		}
	}()

	return &pb.ReceiptPayResponse{
		Pan:          projectCard.Pan,
		Amount:       req.Amount,
		Status:       comment,
		Organization: organization,
	}, nil
}

func (s *BillingService) DeleteProjectCard(ctx context.Context, req *pb.PrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.DeleteProjectCard", req)
	defer dbSpan.Finish()

	s.logger.Info("--DeleteProjectCard-- requested")

	projectCard, err := s.storage.Billing().GetProjectCard(ctx, req)
	if err != nil {
		s.logger.Error("--DeleteProjectCard--get project card", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	payload := map[string]any{
		"id":     helper.GenerateRandomNumber(6),
		"method": config.CardRemove,
		"params": map[string]any{
			"token": projectCard.PaymeToken,
		},
	}
	xauth := fmt.Sprintf("%v:%v", s.config.PaymeXAuth, s.config.PaymeKey)
	cardRemoveResp, err := sendRequest[models.CardRemoveResponse](payload, s.config.PaymeApiUrl, xauth)
	if err != nil {
		s.logger.Error("--DeleteProjectCard--card remove", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	if cardRemoveResp.Error != nil {
		s.logger.Error("--DeleteProjectCard--card remove resp", l.Error(errors.New(cardRemoveResp.Error.Message)))
		return nil, status.Error(codes.Internal, cardRemoveResp.Error.Message)
	}

	err = s.storage.Billing().DeleteProjectCard(ctx, req)
	if err != nil {
		s.logger.Error("--DeleteProjectCard--delete project card", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func sendRequest[T any](payload map[string]any, apiUrl, xauth string) (*T, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-auth", xauth)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var responseData T
	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		return nil, err
	}

	return &responseData, nil
}
