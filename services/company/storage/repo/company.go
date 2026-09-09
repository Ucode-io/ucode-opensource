package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"google.golang.org/protobuf/types/known/emptypb"
)

type CompanyStorageI interface {
	Create(ctx context.Context, company *pb.CreateCompanyRequest) (*pb.CreateCompanyResponse, error)
	Update(ctx context.Context, company *pb.Company) (*pb.Company, error)
	Delete(ctx context.Context, id string) error
	GetList(ctx context.Context, company *pb.GetCompanyListRequest) (*pb.GetComanyListResponse, error)
	GetById(ctx context.Context, id string) (*pb.GetCompanyByIdResponse, error)
	GetListWithProjects(ctx context.Context, queryParam *pb.GetListWithProjectsRequest) (*pb.GetListWithProjectsResponse, error)
	GetAllMenuTemplate(ctx context.Context, req *emptypb.Empty) (*pb.GatAllMenuTemplateResponse, error)
	GetMenuTemplateById(ctx context.Context, req *pb.GetMenuTemplateRequest) (*pb.MenuTemplate, error)
	DeleteAfterTest(ctx context.Context, id string) (rowsAffected int64, err error)
	CreateMenuTemplate(ctx context.Context, req *pb.CreateMenuTemplateRequest) error
	GetProjectMenuTemplates(ctx context.Context, req *pb.GetProjectMenuTemplateRequest) (resp *pb.GetProjectMenuTemplateResponse, err error)
}
