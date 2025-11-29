package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"potter/framework/eventsourcing"
)

// Handler HTTP handlers для работы с банковскими счетами
type Handler struct {
	repo *eventsourcing.EventSourcedRepository[*BankAccountAggregate]
}

// NewHandler создает новый handler
func NewHandler(repo *eventsourcing.EventSourcedRepository[*BankAccountAggregate]) *Handler {
	return &Handler{repo: repo}
}

// CreateAccount создает новый счет
func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AccountNumber string `json:"account_number"`
		OwnerName     string `json:"owner_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	agg := NewBankAccountAggregate(req.AccountNumber)
	agg.OpenAccount(req.AccountNumber, req.OwnerName)

	if err := h.repo.Save(ctx, agg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save account: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"account_number": req.AccountNumber,
		"owner_name":     req.OwnerName,
		"status":         "created",
	})
}

// HandleAccount обрабатывает запросы к конкретному счету
func (h *Handler) HandleAccount(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/accounts/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 {
		http.Error(w, "Account number required", http.StatusBadRequest)
		return
	}

	accountNumber := parts[0]
	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		h.GetAccount(w, r, accountNumber, ctx)
	case http.MethodPost:
		if len(parts) < 2 {
			http.Error(w, "Action required", http.StatusBadRequest)
			return
		}
		action := parts[1]
		switch action {
		case "deposit":
			h.Deposit(w, r, accountNumber, ctx)
		case "withdraw":
			h.Withdraw(w, r, accountNumber, ctx)
		case "close":
			h.CloseAccount(w, r, accountNumber, ctx)
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// GetAccount получает информацию о счете
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request, accountNumber string, ctx interface{}) {
	agg, err := h.repo.GetByID(r.Context(), accountNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Account not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"account_number": agg.GetAccountNumber(),
		"owner_name":     agg.GetOwnerName(),
		"balance":        agg.GetBalance(),
		"is_active":      agg.IsActive(),
		"version":        agg.Version(),
	})
}

// Deposit пополняет счет
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request, accountNumber string, ctx interface{}) {
	var req struct {
		Amount int64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	agg, err := h.repo.GetByID(r.Context(), accountNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Account not found: %v", err), http.StatusNotFound)
		return
	}

	if err := agg.Deposit(req.Amount); err != nil {
		http.Error(w, fmt.Sprintf("Failed to deposit: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.repo.Save(r.Context(), agg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "deposited",
		"amount": req.Amount,
		"balance": agg.GetBalance(),
	})
}

// Withdraw снимает деньги со счета
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request, accountNumber string, ctx interface{}) {
	var req struct {
		Amount int64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	agg, err := h.repo.GetByID(r.Context(), accountNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Account not found: %v", err), http.StatusNotFound)
		return
	}

	if err := agg.Withdraw(req.Amount); err != nil {
		http.Error(w, fmt.Sprintf("Failed to withdraw: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.repo.Save(r.Context(), agg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "withdrawn",
		"amount": req.Amount,
		"balance": agg.GetBalance(),
	})
}

// CloseAccount закрывает счет
func (h *Handler) CloseAccount(w http.ResponseWriter, r *http.Request, accountNumber string, ctx interface{}) {
	agg, err := h.repo.GetByID(r.Context(), accountNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Account not found: %v", err), http.StatusNotFound)
		return
	}

	if err := agg.Close(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to close: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.repo.Save(r.Context(), agg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "closed",
	})
}

