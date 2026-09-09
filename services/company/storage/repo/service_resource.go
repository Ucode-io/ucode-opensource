package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
)

type ServiceResourceStorageI interface {
	GetSingle(ctx context.Context, req *pb.GetSingleServiceResourceReq) (*pb.ServiceResourceModel, error)
	Update(ctx context.Context, req *pb.UpdateServiceResourceReq) (*pb.UpdateServiceResourceRes, error)
	GetList(ctx context.Context, req *pb.GetListServiceResourceReq) (*pb.GetListServiceResourceRes, error)
}
