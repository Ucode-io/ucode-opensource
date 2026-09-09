package repo

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"google.golang.org/protobuf/types/known/emptypb"
)

type IntegrationResourceStorageI interface {
	Create(ctx context.Context, req *pb.CreateIntegrationResourceRequest) (*pb.IntegrationResource, error)
	Upsert(ctx context.Context, req *pb.CreateIntegrationResourceRequest) (*pb.IntegrationResource, error)
	GetById(ctx context.Context, req *pb.IntegrationResourcePrimaryKey) (*pb.IntegrationResource, error)
	GetByUsername(ctx context.Context, req *pb.GetByUsernameRequest) (*pb.GetByUsernameResponse, error)
	GetList(ctx context.Context, req *pb.GetListIntegrationResourceRequest) (*pb.GetListIntegrationResourceResponse, error)
	Update(ctx context.Context, req *pb.UpdateIntegrationResourceRequest) (*emptypb.Empty, error)
	Delete(ctx context.Context, req *pb.IntegrationResourcePrimaryKey) (*emptypb.Empty, error)
}
