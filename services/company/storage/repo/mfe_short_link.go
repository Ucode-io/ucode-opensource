package repo

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"google.golang.org/protobuf/types/known/emptypb"
)

type MfeShortLinkStorageI interface {
	Create(ctx context.Context, req *pb.MfeShortLink) (*pb.MfeShortLink, error)
	GetBySlug(ctx context.Context, req *pb.MfeShortLinkSlugReq) (*pb.MfeShortLink, error)
	GetByFunctionId(ctx context.Context, req *pb.MfeShortLinkFunctionReq) (*pb.MfeShortLink, error)
	DeleteByProjectId(ctx context.Context, req *pb.MfeShortLinkProjectReq) (*emptypb.Empty, error)
}
