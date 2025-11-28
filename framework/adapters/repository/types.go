// Package repository предоставляет generic адаптеры для работы с различными storage backends.
package repository

import "context"

// Entity интерфейс для entity с ID
type Entity interface {
	ID() string
}

// Repository интерфейс для репозитория
type Repository[T Entity] interface {
	Save(ctx context.Context, entity T) error
	FindByID(ctx context.Context, id string) (T, error)
	FindAll(ctx context.Context) ([]T, error)
	Delete(ctx context.Context, id string) error
}

