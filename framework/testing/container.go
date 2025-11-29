package testing

import (
	"context"
	"testing"

	"github.com/akriventsev/potter/framework/container"
)

// NewTestContainer создает тестовый контейнер с дефолтными настройками
// Если сборка контейнера завершается с ошибкой, тест завершается с t.Fatalf
func NewTestContainer(t *testing.T) *container.Container {
	builder := container.NewContainerBuilder(&container.Config{}).
		WithDefaults()

	cnt, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("failed to build test container: %v", err)
	}
	return cnt
}

