package service

import "github.com/kubediscovery/example-service/internal/core/example/entity"

// Service implementa regras de negócio do domínio.
type Service interface {
	GetByID(id string) (*entity.ExampleEntity, error)
}
