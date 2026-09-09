package service

import (
	"context"
	"sync"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"

	"google.golang.org/protobuf/types/known/emptypb"
)

type ServiceNodesI interface {
	Get(namespace string) (client.SharedServiceManagerI, error)
	Add(s client.SharedServiceManagerI, namespace string) error
	Remove(namespace string) error
	SetConfigs(map[string]config.Config)
	GetByNodeType(namespace string, nodeType string) (client.SharedServiceManagerI, error)
}

type serviceNodes struct {
	ServicePool map[string]client.SharedServiceManagerI
	Mu          sync.Mutex
	Configs     map[string]config.Config
}

func NewServiceNodes() ServiceNodesI {
	p := serviceNodes{
		ServicePool: make(map[string]client.SharedServiceManagerI),
		Mu:          sync.Mutex{},
	}

	return &p
}

func (p *serviceNodes) SetConfigs(cfgs map[string]config.Config) {
	p.Configs = cfgs
}

func (p *serviceNodes) GetByNodeType(namespace string, nodeType string) (client.SharedServiceManagerI, error) {
	if nodeType != config.ENTER_PRICE_TYPE {
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

func (p *serviceNodes) Get(namespace string) (client.SharedServiceManagerI, error) {
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

func (p *serviceNodes) Add(s client.SharedServiceManagerI, namespace string) error {
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

func EnterPriceProjectsGrpcSvcs(ctx context.Context, serviceNodes ServiceNodesI, services client.ServiceManagerI, log logger.LoggerI) (ServiceNodesI, map[string]config.Config, error) {
	epProjects, err := services.ProjectServiceClient().GetProjectConfigList(
		ctx,
		&emptypb.Empty{},
	)
	if err != nil {
		log.Error("Error getting enter prise project. GetList", logger.Error(err))
	}

	if epProjects != nil {
		mapProjectConf := map[string]config.Config{}

		for _, v := range epProjects.Configs {
			projectConf := config.Config{

				ObjectBuilderServiceHost: v.OBJECT_BUILDER_SERVICE_HOST,
				ObjectBuilderGRPCPort:    v.OBJECT_BUILDER_GRPC_PORT,

				HighObjectBuilderServiceHost: v.OBJECT_BUILDER_SERVICE_HIGHT_HOST,
				HighObjectBuilderGRPCPort:    v.OBJECT_BUILDER_HIGH_GRPC_PORT,

				SmsServiceHost: v.SMS_SERVICE_HOST,
				SmsGRPCPort:    v.SMS_GRPC_PORT,
			}

			grpcSvcs, err := client.NewSharedGrpcClients(ctx, projectConf)
			if err != nil {
				log.Error("Error connecting grpc client "+v.ProjectId, logger.Error(err))
			}

			if grpcSvcs == nil {
				continue
			}

			err = serviceNodes.Add(grpcSvcs, v.ProjectId)
			if err != nil {
				log.Error("Error adding to grpc pooling enter prise project. ServiceNode "+v.ProjectId, logger.Error(err))
			}

			log.Info(" --- " + v.ProjectId + " --- added to serviceNodes")

			mapProjectConf[v.ProjectId] = projectConf
		}

		return serviceNodes, mapProjectConf, nil
	} else {
		return nil, nil, nil
	}
}
