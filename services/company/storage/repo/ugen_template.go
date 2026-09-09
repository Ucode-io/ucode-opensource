package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
)

type UgenTemplateStorageI interface {
	Create(ctx context.Context, req *pb.CreateUgenTemplateReq) (*pb.UgenTemplate, error)
	GetById(ctx context.Context, id, userID string) (*pb.UgenTemplate, error)
	GetList(ctx context.Context, req *pb.GetUgenTemplateListReq) (*pb.GetUgenTemplateListResponse, error)
	Update(ctx context.Context, req *pb.UpdateUgenTemplateReq) (*pb.UgenTemplate, error)
	SetPrice(ctx context.Context, req *pb.SetUgenTemplatePriceReq) (*pb.UgenTemplate, error)
	Delete(ctx context.Context, id string) error
	SetReaction(ctx context.Context, req *pb.SetUgenTemplateReactionReq) (*pb.UgenTemplateReaction, error)
	DeleteReaction(ctx context.Context, req *pb.DeleteUgenTemplateReactionReq) error
	ListReactions(ctx context.Context, req *pb.GetUgenTemplateReactionListReq) (*pb.GetUgenTemplateReactionListResponse, error)
}
