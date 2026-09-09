package service

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pba "github.com/Ucode-io/ucode-opensource/services/company/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/company/genproto/new_object_builder_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/company/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/company/models"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AdminService ...
type EnvironmentService struct {
	storage        storage.StorageI
	services       ServiceNodesI
	logger         l.Logger
	resource       *ResourceService
	serviceManager client.ServiceManagerI
	cfg            config.BaseConfig
	pb.UnimplementedEnvironmentServiceServer
}

// NewAdminService ...
func NewEnvironmentService(strg storage.StorageI, log l.Logger, services ServiceNodesI, resource *ResourceService, serviceManager client.ServiceManagerI, cfg config.BaseConfig) *EnvironmentService {
	return &EnvironmentService{
		storage:        strg,
		services:       services,
		logger:         log,
		resource:       resource,
		serviceManager: serviceManager,
		cfg:            cfg,
	}
}

func (s *EnvironmentService) Create(ctx context.Context, in *pb.CreateEnvironmentRequest) (*pb.Environment, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_envoirment.Create", in)
	defer dbSpan.Finish()

	s.logger.Info("--CreateEnvironment-- requested", l.Any("request", in))

	resp, err := s.storage.Environment().Create(ctx, in)
	if err != nil {
		s.logger.Error("--CreateEnvironment--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *EnvironmentService) CreateV2(ctx context.Context, in *pb.CreateEnvironmentRequest) (*pb.Environment, error) {
	s.logger.Info("--CreateEnvironment-- requested", l.Any("request", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_envoirment.CreateV2", in)
	defer dbSpan.Finish()

	environments, err := s.storage.Environment().GetList(ctx, &pb.GetEnvironmentListRequest{
		ProjectId: in.GetProjectId(),
	})

	if environments.Count >= 1 && s.cfg.Environment != config.PRODUCTION {
		s.logger.Error("--CreateEnvironment--", l.String("error", "only one environment allowed"))
		return nil, status.Error(codes.Internal, "only one environment allowed")
	}

	resp, err := s.storage.Environment().Create(ctx, in)
	if err != nil {
		s.logger.Error("--CreateEnvironment--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = s.serviceManager.ApiKeysService().Create(ctx, &pba.CreateReq{
		Name:             "Function",
		ProjectId:        in.GetProjectId(),
		EnvironmentId:    resp.GetId(),
		ClientPlatformId: config.OpenFaaSPlatformID,
		Disable:          true,
		ClientTypeId:     in.GetClientTypeId(),
		RoleId:           in.GetRoleId(),
	})
	if err != nil {
		s.logger.Error("--CreateEnvironment--CreateApiKey", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = s.serviceManager.UserService().AddUserToProject(ctx, &pba.AddUserToProjectReq{
		CompanyId:    in.GetCompanyId(),
		ProjectId:    in.GetProjectId(),
		UserId:       in.GetUserId(),
		EnvId:        resp.GetId(),
		ClientTypeId: in.GetClientTypeId(),
		RoleId:       in.GetRoleId(),
	})
	if err != nil {
		s.logger.Error("--CreateEnvironment--AddUserToProject", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	resource, err := s.resource.CreateResource(ctx,
		&pb.CreateResourceReq{
			CompanyId:     in.GetCompanyId(),
			EnvironmentId: resp.GetId(),
			ProjectId:     in.GetProjectId(),
			Resource: &pb.Resource{
				ResourceType: 3,
				NodeType:     config.LowNodeType,
			},
			UserId:       in.GetUserId(),
			NodeType:     config.LowNodeType,
			ClientTypeId: in.GetClientTypeId(),
			RoleId:       in.GetRoleId(),
		},
	)
	if err != nil {
		s.logger.Error("--CreateEnvironment--CreateResource", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = s.storage.ServiceResource().Update(ctx,
		&pb.UpdateServiceResourceReq{
			EnvironmentId:    resp.GetId(),
			ProjectId:        in.GetProjectId(),
			ServiceResources: helper.MakeBodyServiceResource(resource.GetId()),
		},
	)
	if err != nil {
		s.logger.Error("--CreateEnvironment--AddServiceResource", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *EnvironmentService) GetList(ctx context.Context, in *pb.GetEnvironmentListRequest) (*pb.GetEnvironmentListResponse, error) {
	s.logger.Info("--GetEnvironmentList-- requested")

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_envoirment.GetList", in)
	defer dbSpan.Finish()

	environments, err := s.storage.Environment().GetList(ctx, in)
	if err != nil {
		s.logger.Error("--GetEnvironmentList--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if in.WithClientType {
		clientType, err := s.serviceManager.UserService().GetUserProjectClientTypes(
			ctx, &pba.UserInfoPrimaryKey{
				UserId:    in.GetUserId(),
				ProjectId: in.GetProjectId(),
			},
		)
		if err != nil {
			s.logger.Error("--GetEnvironmentList--", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		structReq := map[string]any{
			"limit":  100,
			"offset": 0,
		}

		structReq["guid"] = clientType.ClientTypeIds

		structData, err := helper.ConvertRequestToSturct(structReq)

		for _, env := range environments.GetEnvironments() {
			services, err := s.services.GetByNodeType(in.ProjectId, env.NodeType)
			if err != nil {
				s.logger.Error("-- Get from serviceNode --", l.Error(err))
				return nil, err
			}

			var clientTypes *structpb.Struct

			switch env.ResourceType {
			case 1:
				result, err := services.GetObjectBuilderServiceByType(env.NodeType).GetListSlim(ctx,
					&pbObject.CommonMessage{
						TableSlug: "client_type",
						Data:      structData,
						ProjectId: env.ResourceEnvironmentId,
					})
				if err != nil {
					s.logger.Error("!!!GetClientTypeList.ObjectBuilderService.GetList--->", logger.Error(err))
					return nil, err
				}
				clientTypes = result.Data
			case 3:
				result, err := services.GoObjectBuilderService().GetList2(ctx,
					&nobs.CommonMessage{
						TableSlug: "client_type",
						Data:      structData,
						ProjectId: env.ResourceEnvironmentId,
					})

				if err != nil {
					s.logger.Error("!!!GetClientTypeList.PostgresObjectBuilderService.GetList--->", logger.Error(err))
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}

				clientTypes = result.Data
			}

			env.ClientTypes = clientTypes
		}
	}

	return environments, nil
}

func (s *EnvironmentService) GetById(ctx context.Context, in *pb.EnvironmentPrimaryKey) (*pb.Environment, error) {
	s.logger.Info("--GetEnvironmentById-- requested")

	company, err := s.storage.Environment().GetById(ctx, in)
	if err != nil {
		s.logger.Error("--GetEnvironmentById--", l.Error(err))
	}

	return company, nil
}

func (s *EnvironmentService) Update(ctx context.Context, in *pb.Environment) (*pb.Environment, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_envoirment.Update", in)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateEnvironment-- requested")

	structData, err := helper.ConvertStructToResponse(in.Data)
	if err != nil {
		s.logger.Error("--UpdateEnvironment--ConvertData", l.Error(err))
	}

	payload := models.Environment{
		Id:           in.Id,
		ProjectId:    in.ProjectId,
		Name:         in.Name,
		DisplayColor: in.DisplayColor,
		Description:  in.Description,
		Data:         structData,
	}

	company, err := s.storage.Environment().Update(ctx, payload)
	if err != nil {
		s.logger.Error("--UpdateEnvironment--", l.Error(err))
	}

	return company, nil
}

func (s *EnvironmentService) Delete(ctx context.Context, in *pb.EnvironmentPrimaryKey) (*pb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_envoirment.Delete", in)
	defer dbSpan.Finish()

	s.logger.Info("--DeleteEnvironment-- requested")

	res, err := s.storage.Environment().Delete(ctx, in)
	if err != nil {
		s.logger.Error("--DeleteEnvironment--", l.Error(err))
	}

	return res, nil
}
