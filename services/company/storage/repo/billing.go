package repo

import (
	"context"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
)

type ProjectUserCountFunc func(ctx context.Context, projectId string) (int32, error)

type BillingStorageI interface {
	CreateFare(ctx context.Context, fare *pb.CreateFareRequest) (*pb.Fare, error)
	GetFareById(ctx context.Context, req *pb.PrimaryKey) (*pb.Fare, error)
	GetListFares(ctx context.Context, queryParam *pb.ListRequest) (*pb.ListFareResponse, error)
	UpdateFare(ctx context.Context, fare *pb.Fare) (*pb.Fare, error)
	DeleteFare(ctx context.Context, req *pb.PrimaryKey) error

	CreateFareItem(ctx context.Context, item *pb.CreateFareItemRequest) (*pb.FareItem, error)
	GetFareItem(ctx context.Context, req *pb.PrimaryKey) (*pb.FareItem, error)
	ListFareItem(ctx context.Context, queryParam *pb.ListRequest) (*pb.ListFareItemsResponse, error)
	UpdateFareItem(ctx context.Context, item *pb.FareItem) (*pb.FareItem, error)
	DeleteFareItem(ctx context.Context, req *pb.PrimaryKey) error

	CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.Transaction, error)
	ApplyTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.Transaction, error)
	GetTransactionById(ctx context.Context, req *pb.PrimaryKey) (*pb.Transaction, error)
	GetTransactionByExternalId(ctx context.Context, externalId string) (*pb.Transaction, error)
	GetListTransactions(ctx context.Context, queryParam *pb.ListRequest) (*pb.ListTransactionsResponse, error)
	UpdateTransaction(ctx context.Context, req *pb.Transaction) (*pb.Transaction, error)

	// Ipak Yo'li confirmation. MarkIpakTransactionAccepted atomically flips a still
	// -pending transaction to accepted and credits the project balance, guarded so
	// concurrent callback/poll/cron confirms credit exactly once (returns whether
	// this call performed the credit). MarkIpakTransactionCancelled flips a pending
	// transaction to cancelled. ListPendingIpakTransactions feeds the reconcile cron.
	MarkIpakTransactionAccepted(ctx context.Context, externalId string) (*pb.Transaction, bool, error)
	MarkIpakTransactionCancelled(ctx context.Context, externalId, comment string) error
	ListPendingIpakTransactions(ctx context.Context, olderThan time.Duration) ([]*pb.Transaction, error)

	CompareValue(ctx context.Context, req *pb.CompareFunctionRequest) (*pb.CompareFunctionResponse, error)
	UpsertMonthlyRequest(ctx context.Context, req *pb.MonthlyRequest) error
	CalculatePrice(ctx context.Context, req *pb.CalculatePriceRequest) (*pb.CalculatePriceResponse, error)

	ProcessUcodeRenewals(ctx context.Context) error
	ProcessUgenRenewals(ctx context.Context) error
	ProcessUserSeatRenewals(ctx context.Context, countProjectUsers ProjectUserCountFunc) error
	InactivateProjects(ctx context.Context) error
	DeactivateDeadProjects(ctx context.Context) error

	CreateCard(ctx context.Context, req *pb.CreateProjectCardRequest) (*pb.ProjectCard, error)
	GetProjectCard(ctx context.Context, req *pb.PrimaryKey) (*pb.ProjectCard, error)
	UpdateProjectCard(ctx context.Context, req *pb.ProjectCard) (*pb.ProjectCard, error)
	ListProjectCards(ctx context.Context, req *pb.ListRequest) (*pb.ListProjectCardsResponse, error)
	DeleteProjectCard(ctx context.Context, req *pb.PrimaryKey) error

	ListDiscounts(ctx context.Context, req *pb.ListRequest) (*pb.ListDiscountsResponse, error)
	ListBillingPeriods(ctx context.Context, req *pb.ListBillingPeriodsRequest) (*pb.ListBillingPeriodsResponse, error)

	CreateSubscription(ctx context.Context, req *pb.Subscription) (*pb.Subscription, error)
	GetSubscriptionById(ctx context.Context, req *pb.PrimaryKey) (*pb.Subscription, error)
	UpdateSubscription(ctx context.Context, req *pb.Subscription) (*pb.Subscription, error)
	CancelSubscription(ctx context.Context, req *pb.CancelSubscriptionRequest) (*pb.Subscription, error)
	GetSubscriptionByProjectId(ctx context.Context, req string) (string, error)
	GetProjectBillingStatus(ctx context.Context, req *pb.GetProjectBillingStatusRequest) (*pb.GetProjectBillingStatusResponse, error)

	AddBalanceToProject(ctx context.Context, projectId string, amount float64) error

	UpdateSubscriptionEndDate(ctx context.Context, req *pb.UpdateSubscriptionEndDateReq) (*pb.UpdateSubscriptionEndDateResp, error)
	GetPricingLimits(ctx context.Context, req *pb.GetPricingLimitsRequest) (*pb.GetPricingLimitsResponse, error)
	LogUsage(ctx context.Context, req *pb.LogUsageRequest) error
	GetApiCallMonitoringMetrics(ctx context.Context, req *pb.GetApiCallMonitoringMetricsRequest) (*pb.GetApiCallMonitoringMetricsResponse, error)

	RecordAiTokenUsage(ctx context.Context, req *pb.RecordAiTokenUsageRequest) error

	GetAiTokenUsageMetrics(ctx context.Context, req *pb.GetAiTokenUsageMetricsRequest) (*pb.GetAiTokenUsageMetricsResponse, error)

	CreateTokenPack(ctx context.Context, req *pb.CreateTokenPackRequest) (*pb.TokenPack, error)
	UpdateTokenPack(ctx context.Context, req *pb.TokenPack) (*pb.TokenPack, error)
	DeleteTokenPack(ctx context.Context, req *pb.PrimaryKey) error
	ListTokenPacks(ctx context.Context, req *pb.ListTokenPacksRequest) (*pb.ListTokenPacksResponse, error)
	PurchaseTokenPack(ctx context.Context, req *pb.PurchaseTokenPackRequest) (*pb.PurchaseTokenPackResponse, error)
	GetTokenPackBalance(ctx context.Context, req *pb.GetTokenPackBalanceRequest) (*pb.GetTokenPackBalanceResponse, error)

	ChargeProjectBalance(ctx context.Context, req *pb.ChargeProjectBalanceRequest) (*pb.ChargeProjectBalanceResponse, error)
	RefundProjectBalance(ctx context.Context, req *pb.RefundProjectBalanceRequest) (*pb.RefundProjectBalanceResponse, error)
}
