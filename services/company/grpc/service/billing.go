package service

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
)

type BillingService struct {
	storage storage.StorageI
	logger  l.Logger
	config  config.BaseConfig
	pb.UnimplementedBillingServiceServer
}

func NewBillingService(strg storage.StorageI, log l.Logger, config config.BaseConfig) *BillingService {
	return &BillingService{
		storage: strg,
		logger:  log,
		config:  config,
	}
}

// Fare
func (s *BillingService) CreateFare(ctx context.Context, req *pb.CreateFareRequest) (*pb.Fare, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.Create", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().CreateFare(ctx, req)
	if err != nil {
		s.logger.Error("--CreateFare--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetFare(ctx context.Context, req *pb.PrimaryKey) (*pb.Fare, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetFareByID", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetFareById(ctx, req)
	if err != nil {
		s.logger.Error("--GetFareByID--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) ListFares(ctx context.Context, req *pb.ListRequest) (*pb.ListFareResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetListFares", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetListFares(ctx, req)
	if err != nil {
		s.logger.Error("--GetListFares--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpdateFare(ctx context.Context, req *pb.Fare) (*pb.Fare, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpdateFare", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().UpdateFare(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateFare--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) DeleteFare(ctx context.Context, req *pb.PrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.DeleteFare", req)
	defer dbSpan.Finish()

	err := s.storage.Billing().DeleteFare(ctx, req)
	if err != nil {
		s.logger.Error("--DeleteFare--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// FareItem
func (s *BillingService) CreateFareItem(ctx context.Context, req *pb.CreateFareItemRequest) (*pb.FareItem, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CreateFareItem", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().CreateFareItem(ctx, req)
	if err != nil {
		s.logger.Error("--CreateFareItem--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetFareItem(ctx context.Context, req *pb.PrimaryKey) (*pb.FareItem, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetFareItemByID", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetFareItem(ctx, req)
	if err != nil {
		s.logger.Error("--GetFareItemByID--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) ListFareItems(ctx context.Context, req *pb.ListRequest) (*pb.ListFareItemsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetListFareItems", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().ListFareItem(ctx, req)
	if err != nil {
		s.logger.Error("--GetListFareItems--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpdateFareItem(ctx context.Context, req *pb.FareItem) (*pb.FareItem, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpdateFareItem", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().UpdateFareItem(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateFareItem--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) DeleteFareItem(ctx context.Context, req *pb.PrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.DeleteFareItem", req)
	defer dbSpan.Finish()

	err := s.storage.Billing().DeleteFareItem(ctx, req)
	if err != nil {
		s.logger.Error("--DeleteFareItem--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// Transaction
func (s *BillingService) CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.Transaction, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CreateTransaction", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().ApplyTransaction(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrBalanceInsuffient) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		s.logger.Error("--CreateTransaction--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetTransaction(ctx context.Context, req *pb.PrimaryKey) (*pb.Transaction, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetTransactionByID", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetTransactionById(ctx, req)
	if err != nil {
		s.logger.Error("--GetTransactionByID--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) ListTransactions(ctx context.Context, req *pb.ListRequest) (*pb.ListTransactionsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetListTransactions", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetListTransactions(ctx, req)
	if err != nil {
		s.logger.Error("--GetListTransactions--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpdateTransaction(ctx context.Context, req *pb.Transaction) (*pb.Transaction, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpdateTransaction", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().UpdateTransaction(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateTransaction--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) CompareFunction(ctx context.Context, req *pb.CompareFunctionRequest) (*pb.CompareFunctionResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CompareFunction", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().CompareValue(ctx, req)
	if err != nil {
		s.logger.Error("--CompareFunction--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpsertMonthlyRequest(ctx context.Context, req *pb.MonthlyRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpsertMonthlyRequest", req)
	defer dbSpan.Finish()

	err := s.storage.Billing().UpsertMonthlyRequest(ctx, req)
	if err != nil {
		s.logger.Error("--UpsertMonthlyRequest--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *BillingService) CalculatePrice(ctx context.Context, req *pb.CalculatePriceRequest) (*pb.CalculatePriceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CalculatePrice", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().CalculatePrice(ctx, req)
	if err != nil {
		s.logger.Error("--CalculatePrice--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetSubscription(ctx context.Context, req *pb.PrimaryKey) (*pb.Subscription, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetSubscription", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetSubscriptionById(ctx, req)
	if err != nil {
		s.logger.Error("--GetSubscription--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetProjectBillingStatus(ctx context.Context, req *pb.GetProjectBillingStatusRequest) (*pb.GetProjectBillingStatusResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetProjectBillingStatus", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetProjectBillingStatus(ctx, req)
	if err != nil {
		s.logger.Error("--GetProjectBillingStatus--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpdateSubscription(ctx context.Context, req *pb.Subscription) (*pb.Subscription, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpdateSubscription", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().UpdateSubscription(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateSubscription--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) CancelSubscription(ctx context.Context, req *pb.CancelSubscriptionRequest) (*pb.Subscription, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CancelSubscription", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().CancelSubscription(ctx, req)
	if err != nil {
		s.logger.Error("--CancelSubscription--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) ListDiscounts(ctx context.Context, req *pb.ListRequest) (*pb.ListDiscountsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ListDiscounts", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().ListDiscounts(ctx, req)
	if err != nil {
		s.logger.Error("--ListDiscounts--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) ListBillingPeriods(ctx context.Context, req *pb.ListBillingPeriodsRequest) (*pb.ListBillingPeriodsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ListBillingPeriods", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().ListBillingPeriods(ctx, req)
	if err != nil {
		s.logger.Error("--ListBillingPeriods--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpdateSubscriptionEndDate(ctx context.Context, req *pb.UpdateSubscriptionEndDateReq) (*pb.UpdateSubscriptionEndDateResp, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpdateSubscriptionEndDate", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().UpdateSubscriptionEndDate(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateSubscriptionEndDate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetPricingLimits(ctx context.Context, req *pb.GetPricingLimitsRequest) (*pb.GetPricingLimitsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetPricingLimits", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetPricingLimits(ctx, req)
	if err != nil {
		s.logger.Error("--GetPricingLimits--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) LogUsage(ctx context.Context, req *pb.LogUsageRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.LogUsage", req)
	defer dbSpan.Finish()

	err := s.storage.Billing().LogUsage(ctx, req)
	if err != nil {
		s.logger.Error("--LogUsage--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// GetMonitoringMetrics returns the API usage metrics for a project
func (s *BillingService) GetApiCallMonitoringMetrics(ctx context.Context, req *pb.GetApiCallMonitoringMetricsRequest) (*pb.GetApiCallMonitoringMetricsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetMonitoringMetrics", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().GetApiCallMonitoringMetrics(ctx, req)
	if err != nil {
		s.logger.Error("--GetMonitoringMetrics--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) RecordAiTokenUsage(ctx context.Context, req *pb.RecordAiTokenUsageRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.RecordAiTokenUsage", req)
	defer dbSpan.Finish()

	if req.ProjectId == "" && req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id or company_id is required")
	}

	err := s.storage.Billing().RecordAiTokenUsage(ctx, req)
	if err != nil {
		s.logger.Error("--RecordAiTokenUsage--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *BillingService) GetAiTokenUsageMetrics(ctx context.Context, req *pb.GetAiTokenUsageMetricsRequest) (*pb.GetAiTokenUsageMetricsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetAiTokenUsageMetrics", req)
	defer dbSpan.Finish()

	if req.ProjectId == "" && req.CompanyId == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id or company_id is required")
	}

	resp, err := s.storage.Billing().GetAiTokenUsageMetrics(ctx, req)
	if err != nil {
		s.logger.Error("--GetAiTokenUsageMetrics--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

// Token packs

func (s *BillingService) ListTokenPacks(ctx context.Context, req *pb.ListTokenPacksRequest) (*pb.ListTokenPacksResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ListTokenPacks", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().ListTokenPacks(ctx, req)
	if err != nil {
		s.logger.Error("--ListTokenPacks--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) CreateTokenPack(ctx context.Context, req *pb.CreateTokenPackRequest) (*pb.TokenPack, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.CreateTokenPack", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().CreateTokenPack(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrInvalidTokenPackAmount) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		s.logger.Error("--CreateTokenPack--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) UpdateTokenPack(ctx context.Context, req *pb.TokenPack) (*pb.TokenPack, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.UpdateTokenPack", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Billing().UpdateTokenPack(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrInvalidTokenPackAmount) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, config.ErrTokenPackNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		s.logger.Error("--UpdateTokenPack--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) DeleteTokenPack(ctx context.Context, req *pb.PrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.DeleteTokenPack", req)
	defer dbSpan.Finish()

	err := s.storage.Billing().DeleteTokenPack(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrTokenPackNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		s.logger.Error("--DeleteTokenPack--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *BillingService) PurchaseTokenPack(ctx context.Context, req *pb.PurchaseTokenPackRequest) (*pb.PurchaseTokenPackResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.PurchaseTokenPack", req)
	defer dbSpan.Finish()

	if req.GetCompanyId() == "" || req.GetProjectId() == "" || req.GetPackId() == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id, project_id and pack_id are required")
	}

	resp, err := s.storage.Billing().PurchaseTokenPack(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrTokenPackNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, config.ErrBalanceInsuffient) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, config.ErrInvalidTokenPackAmount) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		s.logger.Error("--PurchaseTokenPack--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) GetTokenPackBalance(ctx context.Context, req *pb.GetTokenPackBalanceRequest) (*pb.GetTokenPackBalanceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.GetTokenPackBalance", req)
	defer dbSpan.Finish()

	if req.GetCompanyId() == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}

	resp, err := s.storage.Billing().GetTokenPackBalance(ctx, req)
	if err != nil {
		s.logger.Error("--GetTokenPackBalance--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

// Project balance charge / refund

func (s *BillingService) ChargeProjectBalance(ctx context.Context, req *pb.ChargeProjectBalanceRequest) (*pb.ChargeProjectBalanceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.ChargeProjectBalance", req)
	defer dbSpan.Finish()

	if req.GetProjectId() == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than zero")
	}

	resp, err := s.storage.Billing().ChargeProjectBalance(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrProjectNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, config.ErrBalanceInsuffient) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, config.ErrInvalidChargeAmount) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		s.logger.Error("--ChargeProjectBalance--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *BillingService) RefundProjectBalance(ctx context.Context, req *pb.RefundProjectBalanceRequest) (*pb.RefundProjectBalanceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_billing.RefundProjectBalance", req)
	defer dbSpan.Finish()

	if req.GetProjectId() == "" || req.GetTransactionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "project_id and transaction_id are required")
	}

	resp, err := s.storage.Billing().RefundProjectBalance(ctx, req)
	if err != nil {
		if errors.Is(err, config.ErrProjectNotFound) || errors.Is(err, config.ErrTransactionNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, config.ErrTransactionNotRefundable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		s.logger.Error("--RefundProjectBalance--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}
