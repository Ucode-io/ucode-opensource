package service

import (
	"context"
	"errors"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"github.com/saidamir98/udevs_pkg/logger"
)

// ServiceResourceService ...
type MicroserviceResourceService struct {
	storage  storage.StorageI
	services ServiceNodesI
	logger   l.Logger
	pb.UnimplementedMicroserviceResourceServer
}

// NewMicroserviceResourceService ...
func NewMicroserviceResourceService(strg storage.StorageI, log l.Logger, services ServiceNodesI) *MicroserviceResourceService {
	return &MicroserviceResourceService{
		storage:  strg,
		services: services,
		logger:   log,
	}
}

func (m *MicroserviceResourceService) GetList(ctx context.Context, in *pb.GetListServiceResourceReq) (*pb.GetListServiceResourceRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_service_resource.GetList", in)
	defer dbSpan.Finish()

	m.logger.Info("--GetList--", logger.Any("req", in))

	sResources, err := m.storage.ServiceResource().GetList(ctx, in)
	if err != nil {
		errGetList := errors.New("cant get service resource list")
		m.logger.Error("--GetList--", logger.Error(err))
		return sResources, errGetList
	}

	resources, err := m.storage.Resource().GetResourceList(ctx,
		&pb.GetResourceListRequest{
			ProjectId:     in.GetProjectId(),
			EnvironmentId: in.GetEnvironmentId(),
		},
	)
	if err != nil {
		errGetList := errors.New("cant get resource list")
		m.logger.Error("--GetList--", logger.Error(err))
		return sResources, errGetList
	}

	sResources.Resources = resources.GetResources()
	sResources.ResourceTypes = pb.ResourceType_name

	return sResources, nil
}

func (m *MicroserviceResourceService) GetSingle(ctx context.Context, in *pb.GetSingleServiceResourceReq) (*pb.ServiceResourceModel, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_service_resource.GetSingle", in)
	defer dbSpan.Finish()

	m.logger.Info("--GetSingle--", logger.Any("req", in))

	res, err := m.storage.ServiceResource().GetSingle(ctx, in)
	if err != nil {
		errGetSingle := errors.New("cant get service resource")
		m.logger.Error("--GetSingle--", logger.Error(err))
		return res, errGetSingle
	}

	return res, nil
}

func (m *MicroserviceResourceService) Update(ctx context.Context, in *pb.UpdateServiceResourceReq) (*pb.UpdateServiceResourceRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_service_resource.Update", in)
	defer dbSpan.Finish()

	m.logger.Info("--Update--", logger.Any("req", in))

	res, err := m.storage.ServiceResource().Update(ctx, in)
	if err != nil {
		errUpdate := errors.New("cant update service resource")
		m.logger.Error("--Update--", logger.Error(err))
		return res, errUpdate
	}

	return res, nil
}
