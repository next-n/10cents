package httpx

import (
	"net/http"

	"gateway/internal/repo"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountsHandler struct {
	DB *pgxpool.Pool
}

func (h *AccountsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}

	a, err := repo.GetAccountByID(r.Context(), h.DB, id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "account not found")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"id":                 a.ID,
		"credit_limit_cents": a.CreditLimitCents,
		"balance_cents":      a.BalanceCents,
		"available_cents":    a.CreditLimitCents - a.BalanceCents,
		"attempt_count":      a.AttemptCount,
		"spent_cents":        a.SpentCents,
	})
}
