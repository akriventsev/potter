// Copyright 2024 Potter Framework Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/akriventsev/potter/framework/core"
	"github.com/akriventsev/potter/framework/metrics"
	"github.com/gin-gonic/gin"
)

// SwaggerUIConfig конфигурация для Swagger UI адаптера
type SwaggerUIConfig struct {
	Enabled                bool
	Path                   string
	SpecPath               string
	DeepLinking            bool
	DisplayRequestDuration bool
	ValidateSpec           bool
}

// DefaultSwaggerUIConfig возвращает конфигурацию Swagger UI по умолчанию
func DefaultSwaggerUIConfig() SwaggerUIConfig {
	return SwaggerUIConfig{
		Enabled:                true,
		Path:                   "/swagger",
		SpecPath:               "./api/openapi/openapi.yaml",
		DeepLinking:            true,
		DisplayRequestDuration: true,
		ValidateSpec:           false,
	}
}

// SwaggerUIAdapter адаптер для интеграции Swagger UI с REST транспортом
type SwaggerUIAdapter struct {
	config      SwaggerUIConfig
	specContent []byte
	running     bool
	mu          sync.RWMutex
	metrics     *metrics.Metrics
}

// NewSwaggerUIAdapter создает новый Swagger UI адаптер
func NewSwaggerUIAdapter(config SwaggerUIConfig) (*SwaggerUIAdapter, error) {
	adapter := &SwaggerUIAdapter{
		config:  config,
		running: false,
	}

	// Загрузка OpenAPI спецификации
	if err := adapter.loadSpec(); err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	// Валидация спецификации (опционально)
	if config.ValidateSpec {
		if err := adapter.validateSpec(); err != nil {
			return nil, fmt.Errorf("failed to validate OpenAPI spec: %w", err)
		}
	}

	// Инициализация метрик
	var err error
	adapter.metrics, err = metrics.NewMetrics()
	if err != nil {
		// Не критично, продолжаем без метрик
		adapter.metrics = nil
	}

	return adapter, nil
}

// loadSpec загружает OpenAPI спецификацию из файла
func (s *SwaggerUIAdapter) loadSpec() error {
	// Проверяем абсолютный путь или относительный
	specPath := s.config.SpecPath
	if !filepath.IsAbs(specPath) {
		// Пытаемся найти относительно текущей директории
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		specPath = filepath.Join(cwd, specPath)
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI spec file: %w", err)
	}

	s.mu.Lock()
	s.specContent = data
	s.mu.Unlock()

	return nil
}

// validateSpec валидирует OpenAPI спецификацию
func (s *SwaggerUIAdapter) validateSpec() error {
	// Базовая валидация - проверка что файл не пустой
	if len(s.specContent) == 0 {
		return fmt.Errorf("OpenAPI spec is empty")
	}

	// В реальной реализации здесь можно использовать библиотеку для валидации
	// Например, github.com/getkin/kin-openapi/openapi3
	// Для базовой реализации просто проверяем наличие "openapi:" в начале
	if len(s.specContent) < 10 || string(s.specContent[:10]) != "openapi: 3" {
		return fmt.Errorf("invalid OpenAPI spec format")
	}

	return nil
}

// RegisterRoutes регистрирует маршруты Swagger UI
func (s *SwaggerUIAdapter) RegisterRoutes(router *gin.Engine) {
	swaggerGroup := router.Group(s.config.Path)
	{
		// Endpoint для получения OpenAPI спецификации
		swaggerGroup.GET("/openapi.yaml", s.serveSpec)

		// Swagger UI HTML
		swaggerGroup.GET("/", s.serveUI)

		// Swagger UI index (redirect)
		swaggerGroup.GET("/index.html", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, s.config.Path+"/")
		})
	}
}

// serveSpec отдает OpenAPI спецификацию
func (s *SwaggerUIAdapter) serveSpec(c *gin.Context) {
	s.mu.RLock()
	specContent := s.specContent
	s.mu.RUnlock()

	if s.metrics != nil {
		s.metrics.RecordTransport(c.Request.Context(), "swagger-ui", 0, true)
	}

	c.Data(http.StatusOK, "application/x-yaml", specContent)
}

// serveUI отдает Swagger UI HTML
func (s *SwaggerUIAdapter) serveUI(c *gin.Context) {
	if s.metrics != nil {
		s.metrics.RecordTransport(c.Request.Context(), "swagger-ui", 0, true)
	}

	html := s.generateSwaggerUIHTML()
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// generateSwaggerUIHTML генерирует HTML для Swagger UI
func (s *SwaggerUIAdapter) generateSwaggerUIHTML() string {
	specURL := s.config.Path + "/openapi.yaml"
	deepLinking := "true"
	if !s.config.DeepLinking {
		deepLinking = "false"
	}
	displayRequestDuration := "true"
	if !s.config.DisplayRequestDuration {
		displayRequestDuration = "false"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
  <style>
    html {
      box-sizing: border-box;
      overflow: -moz-scrollbars-vertical;
      overflow-y: scroll;
    }
    *, *:before, *:after {
      box-sizing: inherit;
    }
    body {
      margin:0;
      background: #fafafa;
    }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      const ui = SwaggerUIBundle({
        url: "%s",
        dom_id: '#swagger-ui',
        deepLinking: %s,
        displayRequestDuration: %s,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
    };
  </script>
</body>
</html>`, specURL, deepLinking, displayRequestDuration)
}

// Start запускает адаптер (реализация core.Lifecycle)
func (s *SwaggerUIAdapter) Start(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	return nil
}

// Stop останавливает адаптер (реализация core.Lifecycle)
func (s *SwaggerUIAdapter) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	return nil
}

// IsRunning проверяет, запущен ли адаптер (реализация core.Lifecycle)
func (s *SwaggerUIAdapter) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Name возвращает имя компонента (реализация core.Component)
func (s *SwaggerUIAdapter) Name() string {
	return "swagger-ui-adapter"
}

// Type возвращает тип компонента (реализация core.Component)
func (s *SwaggerUIAdapter) Type() core.ComponentType {
	return core.ComponentTypeTransport
}

// ReloadSpec перезагружает OpenAPI спецификацию из файла
func (s *SwaggerUIAdapter) ReloadSpec() error {
	return s.loadSpec()
}

// GetSpecContent возвращает содержимое OpenAPI спецификации
func (s *SwaggerUIAdapter) GetSpecContent() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.specContent
}

// SetSpecContent устанавливает содержимое OpenAPI спецификации
func (s *SwaggerUIAdapter) SetSpecContent(content []byte) {
	s.mu.Lock()
	s.specContent = content
	s.mu.Unlock()
}
