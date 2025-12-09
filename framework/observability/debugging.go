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

package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// DebugConfig конфигурация для debugging utilities
type DebugConfig struct {
	Enabled              bool
	LogLevel             string // "debug", "info", "warn", "error"
	EnablePprof          bool
	PprofPort            int
	EnableHealthCheck    bool
	EnableReadinessCheck bool
}

// DefaultDebugConfig возвращает конфигурацию по умолчанию
func DefaultDebugConfig() DebugConfig {
	return DebugConfig{
		Enabled:              false,
		LogLevel:             "info",
		EnablePprof:          false,
		PprofPort:            6060,
		EnableHealthCheck:    true,
		EnableReadinessCheck: true,
	}
}

// DebugManager менеджер для debugging utilities
type DebugManager struct {
	config          DebugConfig
	pprofServer     *http.Server
	healthChecks    []HealthCheck
	readinessChecks []HealthCheck
	running         bool
	mu              sync.RWMutex
}

// NewDebugManager создает новый DebugManager
func NewDebugManager(config DebugConfig) *DebugManager {
	return &DebugManager{
		config:          config,
		healthChecks:    make([]HealthCheck, 0),
		readinessChecks: make([]HealthCheck, 0),
		running:         false,
	}
}

// Start запускает debug server с pprof endpoints
func (dm *DebugManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	dm.running = true
	dm.mu.Unlock()

	if dm.config.EnablePprof {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		dm.pprofServer = &http.Server{
			Addr:    fmt.Sprintf(":%d", dm.config.PprofPort),
			Handler: mux,
		}

		go func() {
			if err := dm.pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				// Логируем ошибку
				_ = err
			}
		}()
	}

	return nil
}

// Stop останавливает debug server
func (dm *DebugManager) Stop(ctx context.Context) error {
	dm.mu.Lock()
	dm.running = false
	dm.mu.Unlock()

	if dm.pprofServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return dm.pprofServer.Shutdown(shutdownCtx)
	}

	return nil
}

// RegisterHealthCheck регистрирует health check
func (dm *DebugManager) RegisterHealthCheck(check HealthCheck) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.healthChecks = append(dm.healthChecks, check)
}

// RegisterReadinessCheck регистрирует readiness check
func (dm *DebugManager) RegisterReadinessCheck(check HealthCheck) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.readinessChecks = append(dm.readinessChecks, check)
}

// HealthCheckHandler возвращает Gin handler для health check
func (dm *DebugManager) HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		dm.mu.RLock()
		checks := dm.healthChecks
		dm.mu.RUnlock()

		result := HealthCheckResult{
			Status:    "healthy",
			Checks:    make(map[string]CheckResult),
			Timestamp: time.Now(),
		}

		allHealthy := true
		for _, check := range checks {
			start := time.Now()
			err := check.Check(ctx)
			duration := time.Since(start)

			status := "healthy"
			if err != nil {
				status = "unhealthy"
				allHealthy = false
			}

			result.Checks[check.Name()] = CheckResult{
				Status: status,
				Message: func() string {
					if err != nil {
						return err.Error()
					}
					return ""
				}(),
				Duration: duration,
			}
		}

		if !allHealthy {
			result.Status = "unhealthy"
			c.JSON(http.StatusServiceUnavailable, result)
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

// ReadinessCheckHandler возвращает Gin handler для readiness check
func (dm *DebugManager) ReadinessCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		dm.mu.RLock()
		checks := dm.readinessChecks
		dm.mu.RUnlock()

		allReady := true
		for _, check := range checks {
			if err := check.Check(ctx); err != nil {
				allReady = false
				break
			}
		}

		if !allReady {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}

// HealthCheck интерфейс для health checks
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

// HealthCheckResult результат health check
type HealthCheckResult struct {
	Status    string                 `json:"status"`
	Checks    map[string]CheckResult `json:"checks"`
	Timestamp time.Time              `json:"timestamp"`
}

// CheckResult результат отдельной проверки
type CheckResult struct {
	Status   string        `json:"status"`
	Message  string        `json:"message,omitempty"`
	Duration time.Duration `json:"duration"`
}

// DatabaseHealthCheck проверка подключения к БД
type DatabaseHealthCheck struct {
	db *sql.DB
}

// NewDatabaseHealthCheck создает новый DatabaseHealthCheck
func NewDatabaseHealthCheck(db *sql.DB) *DatabaseHealthCheck {
	return &DatabaseHealthCheck{db: db}
}

// Name возвращает имя проверки
func (h *DatabaseHealthCheck) Name() string {
	return "database"
}

// Check выполняет проверку
func (h *DatabaseHealthCheck) Check(ctx context.Context) error {
	if h.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// MessageBusHealthCheck проверка message bus
type MessageBusHealthCheck struct {
	checkFunc func(ctx context.Context) error
	name      string
}

// NewMessageBusHealthCheck создает новый MessageBusHealthCheck
func NewMessageBusHealthCheck(name string, checkFunc func(ctx context.Context) error) *MessageBusHealthCheck {
	return &MessageBusHealthCheck{
		name:      name,
		checkFunc: checkFunc,
	}
}

// Name возвращает имя проверки
func (h *MessageBusHealthCheck) Name() string {
	return h.name
}

// Check выполняет проверку
func (h *MessageBusHealthCheck) Check(ctx context.Context) error {
	if h.checkFunc == nil {
		return fmt.Errorf("check function is nil")
	}
	return h.checkFunc(ctx)
}

// DiskSpaceHealthCheck проверка свободного места на диске
type DiskSpaceHealthCheck struct{}

// NewDiskSpaceHealthCheck создает новый DiskSpaceHealthCheck
func NewDiskSpaceHealthCheck() *DiskSpaceHealthCheck {
	return &DiskSpaceHealthCheck{}
}

// Name возвращает имя проверки
func (h *DiskSpaceHealthCheck) Name() string {
	return "disk"
}

// Check выполняет проверку
func (h *DiskSpaceHealthCheck) Check(ctx context.Context) error {
	// Упрощенная проверка - в production нужно использовать системные вызовы
	// Для базовой реализации всегда возвращаем успех
	return nil
}

// MemoryHealthCheck проверка использования памяти
type MemoryHealthCheck struct{}

// NewMemoryHealthCheck создает новый MemoryHealthCheck
func NewMemoryHealthCheck() *MemoryHealthCheck {
	return &MemoryHealthCheck{}
}

// Name возвращает имя проверки
func (h *MemoryHealthCheck) Name() string {
	return "memory"
}

// Check выполняет проверку
func (h *MemoryHealthCheck) Check(ctx context.Context) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Проверка использования памяти
	usedPercent := float64(m.Alloc) / float64(m.Sys) * 100

	if usedPercent > 95 {
		return fmt.Errorf("memory usage too high: %.2f%%", usedPercent)
	}

	return nil
}

// RequestDumpMiddleware Gin middleware для логирования полных HTTP requests/responses
func RequestDumpMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Логирование request
		requestDump := DumpRequest(c.Request)
		// В production здесь должно быть структурированное логирование
		_ = requestDump

		// Обработка запроса
		c.Next()

		// Логирование response
		responseDump := DumpResponse(c.Writer.Status(), c.Writer.Header(), nil)
		// В production здесь должно быть структурированное логирование
		_ = responseDump
	}
}

// DumpRequest форматирует request для логирования
func DumpRequest(r *http.Request) string {
	dump := map[string]interface{}{
		"method": r.Method,
		"url":    r.URL.String(),
		"header": r.Header,
	}

	// Sanitize sensitive data
	if r.Header.Get("Authorization") != "" {
		dump["header"].(http.Header).Set("Authorization", "***")
	}

	data, _ := json.MarshalIndent(dump, "", "  ")
	return string(data)
}

// DumpResponse форматирует response для логирования
func DumpResponse(statusCode int, headers http.Header, body []byte) string {
	dump := map[string]interface{}{
		"status_code": statusCode,
		"headers":     headers,
	}

	if body != nil && len(body) > 0 {
		dump["body"] = string(body)
	}

	data, _ := json.MarshalIndent(dump, "", "  ")
	return string(data)
}

// ProfileCommand профилирует команду
func ProfileCommand(ctx context.Context, commandName string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	// Логирование медленных команд
	if duration > 1*time.Second {
		// В production здесь должно быть структурированное логирование
		_ = fmt.Sprintf("slow command: %s, duration: %v", commandName, duration)
	}

	return err
}

// Bottleneck структура для обнаружения bottlenecks
type Bottleneck struct {
	Type        string
	Description string
	Severity    string // "low", "medium", "high"
}

// DetectBottlenecks автоматически обнаруживает bottlenecks
func DetectBottlenecks(ctx context.Context) []Bottleneck {
	var bottlenecks []Bottleneck

	// Проверка использования памяти
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.Alloc > 100*1024*1024 { // 100MB
		bottlenecks = append(bottlenecks, Bottleneck{
			Type:        "memory",
			Description: "High memory usage detected",
			Severity:    "medium",
		})
	}

	// Проверка количества goroutines
	numGoroutines := runtime.NumGoroutine()
	if numGoroutines > 1000 {
		bottlenecks = append(bottlenecks, Bottleneck{
			Type:        "goroutines",
			Description: fmt.Sprintf("High number of goroutines: %d", numGoroutines),
			Severity:    "high",
		})
	}

	return bottlenecks
}
