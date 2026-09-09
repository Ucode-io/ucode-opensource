package service

import (
	"context"
	"sync"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"google.golang.org/protobuf/types/known/emptypb"
)

type ServiceNodesI interface {
	Get(namespace string) (client.ServiceManagerI, error)
	Add(s client.ServiceManagerI, namespace string) error
	Remove(namespace string) error
	SetConfigs(map[string]config.Config)
	GetConfigByNamespace(namespace string, nodeType string) config.Config
	GetByNodeType(namespace string, nodeType string) (client.ServiceManagerI, error)
}

type serviceNodes struct {
	ServicePool map[string]client.ServiceManagerI
	Mu          sync.Mutex
	Configs     map[string]config.Config
}

func NewServiceNodes() ServiceNodesI {
	p := serviceNodes{
		ServicePool: make(map[string]client.ServiceManagerI),
		Mu:          sync.Mutex{},
	}

	return &p
}

func (p *serviceNodes) SetConfigs(cfgs map[string]config.Config) {
	p.Configs = cfgs
}

func (p *serviceNodes) GetConfigByNamespace(namespace string, nodeType string) config.Config {
	if nodeType == config.EnterPriceNodeType {
		return p.Configs[namespace]
	} else {
		return p.Configs[config.BaseLoad().UcodeNamespace]
	}
}

func (p *serviceNodes) GetByNodeType(namespace string, nodeType string) (client.ServiceManagerI, error) {

	if nodeType != config.EnterPriceNodeType {
		if p.ServicePool == nil {
			return nil, config.ErrNilServicePool
		}

		p.Mu.Lock()
		defer p.Mu.Unlock()

		storage, ok := p.ServicePool[config.BaseLoad().UcodeNamespace]
		if !ok {
			return nil, config.ErrNodeNotExists
		}

		return storage, nil
	} else {
		if p.ServicePool == nil {
			return nil, config.ErrNilServicePool
		}

		p.Mu.Lock()
		defer p.Mu.Unlock()

		storage, ok := p.ServicePool[namespace]
		if !ok {
			return nil, config.ErrNodeNotExists
		}

		return storage, nil
	}
}

func (p *serviceNodes) Get(namespace string) (client.ServiceManagerI, error) {
	if p.ServicePool == nil {
		return nil, config.ErrNilServicePool
	}

	p.Mu.Lock()
	defer p.Mu.Unlock()

	storage, ok := p.ServicePool[namespace]
	if !ok {
		return nil, config.ErrNodeNotExists
	}

	return storage, nil
}

func (p *serviceNodes) Add(s client.ServiceManagerI, namespace string) error {
	if p.ServicePool == nil {
		return config.ErrNilServicePool
	}
	if s == nil {
		return config.ErrNilService
	}

	p.Mu.Lock()
	defer p.Mu.Unlock()

	_, ok := p.ServicePool[namespace]
	if ok {
		return config.ErrNodeExists
	}

	p.ServicePool[namespace] = s

	return nil
}

func (p *serviceNodes) Remove(namespace string) error {
	if p.ServicePool == nil {
		return config.ErrNilServicePool
	}

	p.Mu.Lock()
	defer p.Mu.Unlock()

	_, ok := p.ServicePool[namespace]
	if !ok {
		return config.ErrNodeNotExists
	}

	delete(p.ServicePool, namespace)
	return nil
}

func EnterPriceProjectsGrpcSvcs(ctx context.Context, storage storage.StorageI, serviceNodes ServiceNodesI, log logger.Logger) (ServiceNodesI, map[string]config.Config) {
	epProjects, err := storage.Project().GetListProjectConfig(
		ctx,
		&emptypb.Empty{},
	)
	if err != nil {
		log.Error("Error getting enter prise project. GetList", logger.Error(err))
		return nil, nil
	}

	mapProjectConf := map[string]config.Config{}

	for _, v := range epProjects.Configs {
		projectConf := config.Config{
			FunctionServicePort:    v.FUNCTION_GRPC_PORT,
			FunctionServiceHost:    v.FUNCTION_SERVICE_HOST,
			BuilderServicePort:     v.OBJECT_BUILDER_GRPC_PORT,
			BuilderServiceHost:     v.OBJECT_BUILDER_SERVICE_HOST,
			HighBuilderServicePort: v.OBJECT_BUILDER_HIGH_GRPC_PORT,
			HighBuilderServiceHost: v.OBJECT_BUILDER_SERVICE_HIGHT_HOST,

			UcodeMongoHost:     v.MONGO_HOST,
			UcodeMongoPort:     v.MONGO_PORT,
			UcodeMongoPassword: v.COMPANY_SERVICE_MONGO_PASSWORD,
			UcodeMongoUser:     v.COMPANY_SERVICE_MONGO_USER,
		}

		grpcSvcs, err := client.NewGrpcClients(ctx, projectConf)
		if err != nil {
			log.Error("Error connecting grpc client "+v.ProjectId, logger.Error(err))
		}

		err = serviceNodes.Add(grpcSvcs, v.ProjectId)
		if err != nil {
			log.Error("Error adding to grpc pooling enter prise project. ServiceNode "+v.ProjectId, logger.Error(err))
		}

		log.Info(" --- " + v.ProjectId + " --- added to serviceNodes")

		mapProjectConf[v.ProjectId] = projectConf
	}

	return serviceNodes, mapProjectConf
}
