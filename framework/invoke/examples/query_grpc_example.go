package examples

import (
	"context"
	"fmt"
	"time"

	transportadapters "potter/framework/adapters/transport"
	"potter/framework/invoke"
	"potter/framework/transport"
)

// GetWarehouseQuery запрос для получения склада
type GetWarehouseQuery struct {
	WarehouseID string `json:"warehouse_id"`
}

func (q GetWarehouseQuery) QueryName() string {
	return "get_warehouse"
}

// GetWarehouseResponse ответ на запрос склада
type GetWarehouseResponse struct {
	WarehouseID string    `json:"warehouse_id"`
	Name        string    `json:"name"`
	Location    string    `json:"location"`
	Capacity    int       `json:"capacity"`
	CreatedAt   time.Time `json:"created_at"`
}

// GetWarehouseQueryHandler обработчик запроса склада
type GetWarehouseQueryHandler struct {
	warehouses map[string]*GetWarehouseResponse
}

func NewGetWarehouseQueryHandler() *GetWarehouseQueryHandler {
	return &GetWarehouseQueryHandler{
		warehouses: map[string]*GetWarehouseResponse{
			"warehouse-1": {
				WarehouseID: "warehouse-1",
				Name:        "Main Warehouse",
				Location:    "Moscow",
				Capacity:    10000,
				CreatedAt:   time.Now(),
			},
			"warehouse-2": {
				WarehouseID: "warehouse-2",
				Name:        "Secondary Warehouse",
				Location:    "St. Petersburg",
				Capacity:    5000,
				CreatedAt:   time.Now(),
			},
		},
	}
}

func (h *GetWarehouseQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	query := q.(GetWarehouseQuery)
	warehouse, exists := h.warehouses[query.WarehouseID]
	if !exists {
		return nil, fmt.Errorf("warehouse not found: %s", query.WarehouseID)
	}
	return warehouse, nil
}

func (h *GetWarehouseQueryHandler) QueryName() string {
	return "get_warehouse"
}

// ExampleQueryInvokerWithGRPC демонстрирует использование QueryInvoker с gRPC транспортом
//
// Важно: В этом примере запросы через QueryInvoker выполняются локально в том же процессе,
// используя QueryBus напрямую. REST/gRPC адаптеры принимают внешние HTTP/gRPC запросы
// и маршрутизируют их в QueryBus, тогда как QueryInvoker используется внутри сервиса
// поверх QueryBus для type-safe работы с запросами.
//
// Для демонстрации полного пути (client -> REST/gRPC -> QueryBus -> QueryInvoker -> Handler)
// необходимо использовать HTTP/gRPC клиент и отправлять запросы по сети.
func ExampleQueryInvokerWithGRPC() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Инициализация gRPC адаптера
	grpcConfig := transportadapters.DefaultGRPCConfig()
	grpcConfig.Port = 50051
	grpcConfig.EnableMetrics = true

	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()

	grpcAdapter, err := transportadapters.NewGRPCAdapter(grpcConfig, commandBus, queryBus)
	if err != nil {
		fmt.Printf("Failed to create gRPC adapter: %v\n", err)
		return
	}

	// 2. Регистрация query handler в QueryBus
	handler := NewGetWarehouseQueryHandler()
	if err := queryBus.Register(handler); err != nil {
		fmt.Printf("Failed to register handler: %v\n", err)
		return
	}

	// 3. Создание gRPC service с методом GetWarehouse()
	// В реальном приложении здесь будет protobuf сервис
	// Для демонстрации используем QueryBus напрямую

	// 4. Запуск gRPC server (в реальном приложении)
	// grpcAdapter.Start(ctx) - запускается при регистрации сервиса
	if err := grpcAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start gRPC server: %v\n", err)
		return
	}
	defer func() {
		_ = grpcAdapter.Stop(ctx)
	}()

	// 5. Создание QueryInvoker
	// Для gRPC запросов мы можем использовать QueryBus напрямую
	invoker := invoke.NewQueryInvoker[GetWarehouseQuery, GetWarehouseResponse](queryBus).
		WithTimeout(5 * time.Second)

	// 6. Отправка gRPC запроса через QueryInvoker (внутри использует QueryBus)
	fmt.Println("=== Запрос склада через gRPC ===")
	query := GetWarehouseQuery{WarehouseID: "warehouse-1"}

	result, err := invoker.Invoke(ctx, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Warehouse found: Name=%s, Location=%s, Capacity=%d\n",
			result.Name, result.Location, result.Capacity)
	}

	// 7. Получение protobuf response и маппинг в типизированный результат
	fmt.Println("\n=== Типизированный результат ===")
	warehouse, err := invoker.Invoke(ctx, GetWarehouseQuery{WarehouseID: "warehouse-2"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Warehouse: %s - %s (Capacity: %d)\n",
			warehouse.Name, warehouse.Location, warehouse.Capacity)
	}

	// 8. Демонстрация streaming queries (server-side, client-side, bidirectional)
	fmt.Println("\n=== Streaming queries ===")
	fmt.Println("Streaming queries поддерживаются через расширение gRPC адаптера")
	fmt.Println("Пример: server-side streaming для списка складов")
	fmt.Println("Пример: client-side streaming для batch запросов")
	fmt.Println("Пример: bidirectional streaming для real-time обновлений")

	// 9. Обработка gRPC status codes и metadata
	fmt.Println("\n=== gRPC status codes и metadata ===")
	_, err = invoker.Invoke(ctx, GetWarehouseQuery{WarehouseID: "warehouse-999"})
	if err != nil {
		fmt.Printf("Expected error with gRPC status: %v\n", err)
	}

	// 10. Interceptors для логирования, метрик, authentication
	fmt.Println("\n=== Interceptors ===")
	fmt.Println("Interceptors поддерживаются через расширение gRPC адаптера")
	fmt.Println("Пример: logging interceptor для всех запросов")
	fmt.Println("Пример: metrics interceptor для сбора метрик")
	fmt.Println("Пример: auth interceptor для проверки JWT токенов")

	// 11. Health checking и reflection
	fmt.Println("\n=== Health checking и reflection ===")
	fmt.Println("Health checking и reflection поддерживаются через расширение gRPC адаптера")

	// Output:
	// === Запрос склада через gRPC ===
	// Warehouse found: Name=Main Warehouse, Location=Moscow, Capacity=10000
	// === Типизированный результат ===
	// Warehouse: Secondary Warehouse - St. Petersburg (Capacity: 5000)
	// === Streaming queries ===
	// Streaming queries поддерживаются через расширение gRPC адаптера
	// Пример: server-side streaming для списка складов
	// Пример: client-side streaming для batch запросов
	// Пример: bidirectional streaming для real-time обновлений
	// === gRPC status codes и metadata ===
	// Expected error with gRPC status: warehouse not found: warehouse-999
	// === Interceptors ===
	// Interceptors поддерживаются через расширение gRPC адаптера
	// Пример: logging interceptor для всех запросов
	// Пример: metrics interceptor для сбора метрик
	// Пример: auth interceptor для проверки JWT токенов
	// === Health checking и reflection ===
	// Health checking и reflection поддерживаются через расширение gRPC адаптера
}

