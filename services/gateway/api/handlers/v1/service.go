package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
)

func (h *HandlerV1) GetService(namespace string) (services.ServiceManagerI, error) {
	return h.services.Get(namespace)
}

func (h *HandlerV1) RemoveService(namespace string) error {
	return h.services.Remove(namespace)
}

func (h *HandlerV1) IsServiceExists(namespace string) bool {
	_, err := h.services.Get(namespace)

	return err == nil
}
