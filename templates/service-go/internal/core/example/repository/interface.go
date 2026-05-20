package repository

import "github.com/kubediscovery/example-service/internal/core/example/entity"

// Repository abstrai persistência do domínio.
type Repository interface {
	FindByID(id string) (*entity.ExampleEntity, error)
}
