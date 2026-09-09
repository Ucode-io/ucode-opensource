package repo

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/models"
)

type EnvironmentStorageI interface {
	Create(ctx context.Context, in *pb.CreateEnvironmentRequest) (*pb.Environment, error)
	GetList(ctx context.Context, in *pb.GetEnvironmentListRequest) (*pb.GetEnvironmentListResponse, error)
	GetById(ctx context.Context, in *pb.EnvironmentPrimaryKey) (*pb.Environment, error)
	Update(ctx context.Context, in models.Environment) (*pb.Environment, error)
	Delete(ctx context.Context, in *pb.EnvironmentPrimaryKey) (*pb.Empty, error)
	GetListWithPKs(ctx context.Context, in []string) (*pb.GetEnvironmentListResponse, error)
}
