package service

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	LimitCodeUsers    = "user_limit"
	LimitCodeBuilders = "builders_limit"
)

func checkUserProjectLimit(ctx context.Context, services client.ServiceManagerI, strg storage.StorageI, fareId, companyId string) error {
	if len(fareId) == 0 {
		return nil
	}

	count, err := strg.User().GetCompanyUsersCount(ctx, companyId)
	if err != nil {
		return status.Error(codes.Internal, "error getting users count")
	}

	if count == 1 {
		return nil
	}

	limitResp, err := services.BillingServiceClient().CompareFunction(ctx, &pbc.CompareFunctionRequest{
		Type:   config.FARE_USERS,
		FareId: fareId,
		Count:  count + 1,
	})
	if err != nil {
		return status.Error(codes.Internal, "error checking user limit")
	}

	if !limitResp.HasAccess {
		return status.Error(codes.ResourceExhausted, LimitCodeUsers+": you have reached the user limit on your current plan. Please upgrade to add more users.")
	}

	return nil
}

func checkUgenBuildersLimit(ctx context.Context, services client.ServiceManagerI, strg storage.StorageI, fareId, projectId string) error {
	if len(fareId) == 0 {
		return nil
	}

	count, err := strg.User().GetProjectUsersCount(ctx, projectId)
	if err != nil {
		return status.Error(codes.Internal, "error getting users count")
	}

	limitResp, err := services.BillingServiceClient().CompareFunction(ctx, &pbc.CompareFunctionRequest{
		Type:   config.FARE_BUILDERS,
		FareId: fareId,
		Count:  count + 1,
	})
	if err != nil {
		return status.Error(codes.Internal, "error checking user limit")
	}

	if !limitResp.HasAccess {
		return status.Error(codes.ResourceExhausted, LimitCodeBuilders+": you have reached the builder user limit on your current plan. Please upgrade to add more builder users.")
	}

	return nil
}
