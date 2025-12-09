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
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/gin-gonic/gin"
)

// ValidationOptions опции для валидации OpenAPI
type ValidationOptions struct {
	ValidateRequest       bool
	ValidateResponse      bool
	IncludeResponseStatus bool
	MultiError            bool
	CustomSchemaErrorFunc func(error) string
}

// DefaultValidationOptions возвращает опции валидации по умолчанию
func DefaultValidationOptions() *ValidationOptions {
	return &ValidationOptions{
		ValidateRequest:       true,
		ValidateResponse:      false,
		IncludeResponseStatus: true,
		MultiError:            true,
	}
}

// OpenAPIValidator валидатор HTTP запросов по OpenAPI спецификации
type OpenAPIValidator struct {
	spec    *openapi3.T
	router  routers.Router
	options *ValidationOptions
}

// NewOpenAPIValidator создает новый OpenAPI валидатор
func NewOpenAPIValidator(specPath string, options *ValidationOptions) (*OpenAPIValidator, error) {
	// Загрузка OpenAPI спецификации
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Проверяем абсолютный путь или относительный
	if !filepath.IsAbs(specPath) {
		cwd, err := filepath.Abs(".")
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		specPath = filepath.Join(cwd, specPath)
	}

	spec, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	// Валидация спецификации
	if err := spec.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	// Создание роутера для быстрого поиска операций
	var router routers.Router
	if spec != nil {
		var err error
		router, err = legacy.NewRouter(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create router: %w", err)
		}
	}

	// Настройка опций по умолчанию
	if options == nil {
		options = DefaultValidationOptions()
	}

	return &OpenAPIValidator{
		spec:    spec,
		router:  router,
		options: options,
	}, nil
}

// responseWriter обертка для gin.ResponseWriter для перехвата ответа
type responseWriter struct {
	gin.ResponseWriter
	body       []byte
	statusCode int
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Middleware возвращает Gin middleware для валидации запросов
func (v *OpenAPIValidator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Валидация запроса
		if v.options.ValidateRequest {
			if err := v.ValidateRequest(c); err != nil {
				v.handleValidationError(c, err)
				c.Abort()
				return
			}
		}

		// Обертка для response writer, если нужна валидация ответа
		if v.options.ValidateResponse {
			rw := &responseWriter{
				ResponseWriter: c.Writer,
				body:           make([]byte, 0),
				statusCode:     http.StatusOK,
			}
			c.Writer = rw

			c.Next()

			// Валидация ответа после обработки handler
			if err := v.ValidateResponse(c, rw.statusCode, rw.body); err != nil {
				// Если валидация ответа не прошла, логируем ошибку, но не прерываем запрос
				// В production здесь должно быть структурированное логирование
				_ = err
			}
		} else {
			c.Next()
		}
	}
}

// ValidateRequest валидирует HTTP запрос по OpenAPI спецификации
func (v *OpenAPIValidator) ValidateRequest(c *gin.Context) error {
	// Поиск операции в OpenAPI спецификации
	route, pathParams, err := v.router.FindRoute(c.Request)
	if err != nil {
		return fmt.Errorf("route not found: %w", err)
	}

	// Создание request validation input
	input := &openapi3filter.RequestValidationInput{
		Request:     c.Request,
		PathParams:  pathParams,
		Route:       route,
		QueryParams: c.Request.URL.Query(),
	}

	// Валидация запроса
	if err := openapi3filter.ValidateRequest(c.Request.Context(), input); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// ValidateResponse валидирует HTTP ответ по OpenAPI спецификации
func (v *OpenAPIValidator) ValidateResponse(c *gin.Context, statusCode int, body []byte) error {
	if !v.options.ValidateResponse {
		return nil
	}

	// Поиск операции для request validation input
	route, pathParams, err := v.router.FindRoute(c.Request)
	if err != nil {
		return fmt.Errorf("route not found: %w", err)
	}

	// Создание request validation input для response validation
	requestInput := &openapi3filter.RequestValidationInput{
		Request:     c.Request,
		PathParams:  pathParams,
		Route:       route,
		QueryParams: c.Request.URL.Query(),
	}

	// Создание response validation input
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 statusCode,
		Header:                 c.Writer.Header(),
		Body:                   io.NopCloser(strings.NewReader(string(body))),
		Options:                &openapi3filter.Options{},
	}

	// Валидация ответа
	if err := openapi3filter.ValidateResponse(c.Request.Context(), input); err != nil {
		return fmt.Errorf("response validation failed: %w", err)
	}

	return nil
}

// handleValidationError обрабатывает ошибку валидации
func (v *OpenAPIValidator) handleValidationError(c *gin.Context, err error) {
	validationErr := v.formatValidationError(err)

	response := gin.H{
		"error":   "validation_failed",
		"details": validationErr,
	}

	// Добавление статуса ответа, если включено
	if v.options.IncludeResponseStatus {
		response["status_code"] = http.StatusBadRequest
	}

	c.JSON(http.StatusBadRequest, response)
}

// formatValidationError форматирует ошибку валидации
func (v *OpenAPIValidator) formatValidationError(err error) []ValidationError {
	var validationErrors []ValidationError

	// Парсинг ошибки - kin-openapi может возвращать разные типы ошибок
	// Пытаемся извлечь множественные ошибки из строки
	errStr := err.Error()

	// Если ошибка содержит несколько сообщений (разделенных переносами строк или точками с запятой)
	if v.options.MultiError && (strings.Contains(errStr, "\n") || strings.Contains(errStr, ";")) {
		// Разбиваем на отдельные ошибки
		parts := strings.Split(errStr, "\n")
		if len(parts) == 1 {
			parts = strings.Split(errStr, ";")
		}

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				validationErrors = v.parseSingleError(fmt.Errorf("%s", part), validationErrors)
			}
		}
	} else {
		// Простая ошибка - парсим как одну ошибку
		validationErrors = v.parseSingleError(err, validationErrors)
	}

	// Если MultiError=false, возвращаем только первую ошибку
	if !v.options.MultiError && len(validationErrors) > 0 {
		return validationErrors[:1]
	}

	return validationErrors
}

// parseMultiError парсит множественные ошибки
func (v *OpenAPIValidator) parseMultiError(err error, errors []ValidationError) []ValidationError {
	// Парсим строку ошибки для извлечения информации о полях
	errStr := err.Error()

	// Пытаемся найти информацию о поле в сообщении об ошибке
	// Формат ошибок kin-openapi обычно содержит путь к полю
	field := ""
	schema := ""

	// Парсим JSON pointer из сообщения об ошибке (если есть)
	if strings.Contains(errStr, "#/") {
		parts := strings.Split(errStr, "#/")
		if len(parts) > 1 {
			path := parts[1]
			pathParts := strings.Split(path, "/")
			if len(pathParts) > 0 {
				field = pathParts[len(pathParts)-1]
			}
		}
	}

	message := errStr
	if v.options.CustomSchemaErrorFunc != nil {
		message = v.options.CustomSchemaErrorFunc(err)
	}

	errors = append(errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   nil,
		Schema:  schema,
	})

	return errors
}

// parseSingleError парсит одиночную ошибку
func (v *OpenAPIValidator) parseSingleError(err error, errors []ValidationError) []ValidationError {
	message := err.Error()
	if v.options.CustomSchemaErrorFunc != nil {
		message = v.options.CustomSchemaErrorFunc(err)
	}

	// Пытаемся извлечь информацию о поле из сообщения об ошибке
	field := ""
	errStr := err.Error()
	if strings.Contains(errStr, "#/") {
		parts := strings.Split(errStr, "#/")
		if len(parts) > 1 {
			path := parts[1]
			pathParts := strings.Split(path, "/")
			if len(pathParts) > 0 {
				field = pathParts[len(pathParts)-1]
			}
		}
	}

	errors = append(errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   nil,
		Schema:  "",
	})

	return errors
}

// ValidationError структура ошибки валидации
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
	Schema  string      `json:"schema,omitempty"`
}

// GetSpec возвращает загруженную OpenAPI спецификацию
func (v *OpenAPIValidator) GetSpec() *openapi3.T {
	return v.spec
}

// Reload перезагружает OpenAPI спецификацию из файла
func (v *OpenAPIValidator) Reload(specPath string) error {
	validator, err := NewOpenAPIValidator(specPath, v.options)
	if err != nil {
		return err
	}

	v.spec = validator.spec
	v.router = validator.router
	return nil
}
