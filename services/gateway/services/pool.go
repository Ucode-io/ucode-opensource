package services

import (
	"sync"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

type ServiceNodesI interface {
	Get(namespace string) (ServiceManagerI, error)
	Add(s ServiceManagerI, namespace string) error
	Remove(namespace string) error
}

type serviceNodes struct {
	ServicePool map[string]ServiceManagerI
	Mu          sync.Mutex
}

func NewServiceNodes() ServiceNodesI {
	p := serviceNodes{
		ServicePool: make(map[string]ServiceManagerI),
		Mu:          sync.Mutex{},
	}

	return &p
}

func (p *serviceNodes) Get(namespace string) (ServiceManagerI, error) {
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

func (p *serviceNodes) Add(s ServiceManagerI, namespace string) error {
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
