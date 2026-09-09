package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	_ "github.com/lib/pq"
)

type ResourceStorageI interface {
	GetResourceList(ctx context.Context, in *pb.GetResourceListRequest) (*pb.GetResourceListResponse, error)
	GetResource(ctx context.Context, in *pb.GetResourceRequest) (*pb.Resource, error)
	AddResource(ctx context.Context, in *pb.Resource) (*pb.Resource, error)
	UpdateResource(ctx context.Context, in *pb.UpdateResourceRequest) (*pb.UpdateResourceResponse, error)
	RemoveResource(ctx context.Context, in *pb.RemoveResourceRequest) (*pb.EmptyProto, error)
	GetResourceWithPath(ctx context.Context, in *pb.GetResourceRequest) (*pb.GetResourceWithPathResponse, error)
	GetResourceManyWithPath(ctx context.Context, in *pb.GetResourceManyRequest) (*pb.GetResourceManyWithPathResponse, error)
	GetResEnvByResIdEnvId(ctx context.Context, in *pb.GetResEnvByResIdEnvIdRequest) (*pb.ResourceEnvironment, error)

	// resource environment
	GetListResourceEnvironment(ctx context.Context, in *pb.GetListResourceEnvironmentReq) ([]*pb.ResourceEnvironment, error)
	GetSingleResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error)
	InsertResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error)
	UpdateResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error)
	RemoveResourceEnvironmentByID(ctx context.Context, in *pb.ResourceEnvironment) (*pb.EmptyProto, error)
	GetListConfiguredResourceEnvironment(ctx context.Context, in *pb.GetListConfiguredResourceEnvironmentReq) (*pb.GetListConfiguredResourceEnvironmentRes, error)
	GetDefaultResource(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error)
	UpdateDefaultResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) ([]*pb.ResourceEnvironment, error)
	GetResourceByEnvID(context.Context, *pb.GetResourceByEnvIDRequest) (*pb.GetResourceByEnvIDResponse, error)
	RemoveResourceEnvironmentByResourceID(context.Context, *pb.ResourceEnvironment) (*pb.EmptyProto, error)
	GetServiceResources(context.Context, *pb.GetServiceResourcesReq) (map[pb.ServiceType][]*pb.GetServiceResourcesRes_ServiceTypeResources_ServiceResources, error)
	SetDefaultResource(context.Context, *pb.SetDefaultResourceReq) (*pb.SetDefaultResourceRes, error)
	RemoveDefaultResource(context.Context, *pb.SetDefaultResourceReq) error
	GetResEnvByResIdEnvIdNotDefault(ctx context.Context, in *pb.GetResEnvByResIdEnvIdRequest) (*pb.ResourceEnvironment, error)

	CreateVariableResource(ctx context.Context, in *pb.CreateVariableResourceRequest) (*pb.VariableResource, error)
	GetVariableResourceList(ctx context.Context, in *pb.GetVariableResourceListRequest) (*pb.GetVariableResourceListResponse, error)
	GetSingleVariableResource(ctx context.Context, in *pb.PrimaryKeyVariableResource) (*pb.VariableResource, error)
	UpdateVariableResource(ctx context.Context, in *pb.VariableResource) (*pb.Empty, error)
	DeleteVariableResource(ctx context.Context, in *pb.PrimaryKeyVariableResource) (*pb.Empty, error)

	AddResourceToProject(ctx context.Context, in *pb.AddResourceToProjectRequest) (*pb.ProjectResource, error)
	UpsertProjectResource(ctx context.Context, in *pb.AddResourceToProjectRequest) (*pb.ProjectResource, error)
	GetProjectResourceList(ctx context.Context, in *pb.GetProjectResourceListRequest) (*pb.ListProjectResource, error)
	GetSingleProjectResouece(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.ProjectResource, error)
	UpdateProjectResource(ctx context.Context, in *pb.ProjectResource) (*pb.Empty, error)
	DeleteProjectResource(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.Empty, error)
	GetProjectResourcesByExternalId(ctx context.Context, in *pb.GetByExternalIdRequest) (*pb.ListProjectResource, error)
}
