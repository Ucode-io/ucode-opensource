package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
)

type TemplateStorageI interface {
	Create(ctx context.Context, template *pb.CreateTemplateMetadataReq) (*pb.TemplateMetadata, error)
	GetById(ctx context.Context, id string) (*pb.TemplateMetadata, error)
	GetList(ctx context.Context, req *pb.GetTemplateMetadataListReq) (*pb.GetTemplateMetadataListResponse, error)
	Update(ctx context.Context, template *pb.UpdateTemplateMetadataReq) (*pb.TemplateMetadata, error)
	Delete(ctx context.Context, id string) error
}
