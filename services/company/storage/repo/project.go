package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/models"

	"google.golang.org/protobuf/types/known/emptypb"
)

type ProjectStorageI interface {
	Create(ctx context.Context, id string, project *pb.CreateProjectRequest) (*pb.CreateProjectResponse, error)
	Update(ctx context.Context, project *pb.Project) (*pb.Project, error)
	Delete(ctx context.Context, id string) error
	GetList(ctx context.Context, project *pb.GetProjectListRequest) (*pb.GetProjectListResponse, error)
	GetById(ctx context.Context, projectId string) (*pb.Project, error)
	GetUgenProjectByCompanyId(ctx context.Context, companyId string) (*pb.Project, error)
	GetProjectsByCompanyId(ctx context.Context, req *pb.GetProjectsByCompanyIdReq) (*pb.GetProjectsByCompanyIdRes, error)
	GetProjectResources(ctx context.Context, in *pb.GetProjectsRequest) (*pb.GetProjectRes, error)
	GetListLanguage(ctx context.Context, in *pb.GetListSettingReq) (*models.ListLanguage, error)
	GetListCurrency(ctx context.Context, in *pb.GetListSettingReq) (*models.ListCurrency, error)
	GetListTimezone(ctx context.Context, in *pb.GetListSettingReq) (*models.ListTimezone, error)
	DeleteAfterTest(ctx context.Context, id string) error
	ListProjectRPS(ctx context.Context, in *pb.GetProjectListRequest) (*pb.ListProjectsRPSResponse, error)

	GetProjectUgenStatus(ctx context.Context, req *pb.GetProjectUgenStatusRequest) (*pb.GetProjectUgenStatusResponse, error)
	UpdateProjectUgenAccess(ctx context.Context, req *pb.UpdateProjectUgenAccessRequest) (*pb.UpdateProjectUgenAccessResponse, error)
	AutoAssignUgenIfSingle(ctx context.Context, req *pb.AutoAssignUgenIfSingleRequest) (*pb.AutoAssignUgenIfSingleResponse, error)
	ListUgenProjects(ctx context.Context, req *pb.ListUgenProjectsRequest) (*pb.ListUgenProjectsResponse, error)
	ListAllUgenProjects(ctx context.Context, search string) ([]*pb.UgenProjectItem, error)

	BindMicroFrontToProject(ctx context.Context, in *pb.ProjectLoginMicroFrontend) (*pb.ProjectLoginMicroFrontend, error)
	GetProjectLoginMicroFront(ctx context.Context, in *pb.GetProjectLoginMicroFrontRequest) (*pb.ProjectLoginMicroFrontend, error)
	UpdateProjectLoginMicroFront(ctx context.Context, in *pb.ProjectLoginMicroFrontend) (*pb.ProjectLoginMicroFrontend, error)

	CreateProjectConfig(ctx context.Context, in *pb.ProjectConfig) (*pb.ProjectConfig, error)
	UpdateProjectConfig(ctx context.Context, in *pb.ProjectConfig) (*pb.ProjectConfig, error)
	GetProjectConfigById(ctx context.Context, in *pb.GetPorjectConfigByProjectIdRequest) (*pb.ProjectConfig, error)
	GetListProjectConfig(ctx context.Context, in *emptypb.Empty) (*pb.ListPorjectConfig, error)

	AttachFare(ctx context.Context, in *pb.AttachFareRequest) (*pb.Project, error)
	AttachCustomer(ctx context.Context, in *pb.AttachCustomerRequest) (*emptypb.Empty, error)
}
