package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"google.golang.org/protobuf/types/known/emptypb"
)

type RedirectStorageI interface {
	Create(ctx context.Context, req *pb.RedirectUrl) (*pb.RedirectUrl, error)
	Update(ctx context.Context, req *pb.RedirectUrl) (*pb.RedirectUrl, error)
	GetSingle(ctx context.Context, req *pb.GetSingleRedirectUrlReq) (*pb.RedirectUrl, error)
	GetList(ctx context.Context, req *pb.GetListRedirectUrlReq) (*pb.GetListRedirectUrlRes, error)
	Delete(ctx context.Context, req *pb.DeleteRedirectUrlReq) (*emptypb.Empty, error)
	UpdateOrder(ctx context.Context, req *pb.UpdateOrderRedirectUrlReq) (*emptypb.Empty, error)
}
