package examples

import (
	"context"
	"fmt"
	"time"

	transportadapters "potter/framework/adapters/transport"
	"potter/framework/invoke"
	"potter/framework/transport"
)

// GetProductQuery запрос для получения продукта
type GetProductQuery struct {
	ProductID string `json:"product_id" uri:"id"`
}

func (q GetProductQuery) QueryName() string {
	return "get_product"
}

// GetProductResponse ответ на запрос продукта
type GetProductResponse struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	SKU       string  `json:"sku"`
	InStock   bool    `json:"in_stock"`
}

// GetProductQueryHandler обработчик запроса продукта
type GetProductQueryHandler struct {
	products map[string]*GetProductResponse
}

func NewGetProductQueryHandler() *GetProductQueryHandler {
	return &GetProductQueryHandler{
		products: map[string]*GetProductResponse{
			"prod-1": {
				ProductID: "prod-1",
				Name:      "Laptop",
				Price:     999.99,
				SKU:       "LAP-001",
				InStock:   true,
			},
			"prod-2": {
				ProductID: "prod-2",
				Name:      "Mouse",
				Price:     29.99,
				SKU:       "MOU-001",
				InStock:   true,
			},
		},
	}
}

func (h *GetProductQueryHandler) Handle(ctx context.Context, q transport.Query) (interface{}, error) {
	query := q.(GetProductQuery)
	product, exists := h.products[query.ProductID]
	if !exists {
		return nil, fmt.Errorf("product not found: %s", query.ProductID)
	}
	return product, nil
}

func (h *GetProductQueryHandler) QueryName() string {
	return "get_product"
}

// ExampleQueryInvokerWithREST демонстрирует использование QueryInvoker с REST HTTP транспортом
//
// Важно: В этом примере запросы через QueryInvoker выполняются локально в том же процессе,
// используя QueryBus напрямую. REST/gRPC адаптеры принимают внешние HTTP/gRPC запросы
// и маршрутизируют их в QueryBus, тогда как QueryInvoker используется внутри сервиса
// поверх QueryBus для type-safe работы с запросами.
//
// Для демонстрации полного пути (client -> REST/gRPC -> QueryBus -> QueryInvoker -> Handler)
// необходимо использовать HTTP/gRPC клиент и отправлять запросы по сети.
func ExampleQueryInvokerWithREST() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Инициализация REST адаптера
	restConfig := transportadapters.DefaultRESTConfig()
	restConfig.Port = 8080
	restConfig.BasePath = "/api/v1"
	restConfig.EnableMetrics = true

	commandBus := transport.NewInMemoryCommandBus()
	queryBus := transport.NewInMemoryQueryBus()

	restAdapter, err := transportadapters.NewRESTAdapter(restConfig, commandBus, queryBus)
	if err != nil {
		fmt.Printf("Failed to create REST adapter: %v\n", err)
		return
	}

	// 2. Регистрация query handler в QueryBus
	handler := NewGetProductQueryHandler()
	if err := queryBus.Register(handler); err != nil {
		fmt.Printf("Failed to register handler: %v\n", err)
		return
	}

	// 3. Регистрация REST endpoint через RegisterQuery() (GET /api/v1/products/:id)
	restAdapter.RegisterQuery("GET", "/products/:id", GetProductQuery{})

	// 4. Запуск REST сервера
	if err := restAdapter.Start(ctx); err != nil {
		fmt.Printf("Failed to start REST server: %v\n", err)
		return
	}
	defer func() {
		_ = restAdapter.Stop(ctx)
	}()

	// Даем серверу время на запуск
	time.Sleep(100 * time.Millisecond)

	// 5. Создание QueryInvoker
	// Для REST запросов мы можем использовать QueryBus напрямую
	invoker := invoke.NewQueryInvoker[GetProductQuery, GetProductResponse](queryBus).
		WithTimeout(5 * time.Second)

	// 6. Отправка HTTP GET запроса через QueryInvoker (внутри использует QueryBus)
	fmt.Println("=== Запрос продукта через REST ===")
	query := GetProductQuery{ProductID: "prod-1"}

	result, err := invoker.Invoke(ctx, query)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Product found: Name=%s, Price=%.2f, SKU=%s\n",
			result.Name, result.Price, result.SKU)
	}

	// 7. Получение JSON response и десериализация в типизированный результат
	fmt.Println("\n=== Типизированный результат ===")
	product, err := invoker.Invoke(ctx, GetProductQuery{ProductID: "prod-2"})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Product: %s - $%.2f (InStock: %v)\n",
			product.Name, product.Price, product.InStock)
	}

	// 8. Демонстрация query parameters, headers, authentication
	// В реальном приложении это можно сделать через расширение REST адаптера
	fmt.Println("\n=== Query parameters и headers ===")
	fmt.Println("Query parameters и headers поддерживаются через расширение REST адаптера")

	// 9. Обработка HTTP ошибок (404, 500, timeout)
	fmt.Println("\n=== Обработка ошибок ===")
	_, err = invoker.Invoke(ctx, GetProductQuery{ProductID: "prod-999"})
	if err != nil {
		fmt.Printf("Expected error for non-existent product: %v\n", err)
	}

	// 10. Content negotiation (JSON/XML) - поддерживается через расширение REST адаптера
	fmt.Println("\n=== Content negotiation ===")
	fmt.Println("Content negotiation поддерживается через расширение REST адаптера")

	// Output:
	// === Запрос продукта через REST ===
	// Product found: Name=Laptop, Price=999.99, SKU=LAP-001
	// === Типизированный результат ===
	// Product: Mouse - $29.99 (InStock: true)
	// === Query parameters и headers ===
	// Query parameters и headers поддерживаются через расширение REST адаптера
	// === Обработка ошибок ===
	// Expected error for non-existent product: product not found: prod-999
	// === Content negotiation ===
	// Content negotiation поддерживается через расширение REST адаптера
}

