package httpx

import (
	"encoding/json"
	"net/http"

	"gateway/internal/domain"
	"gateway/internal/repo"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentIntentsHandler struct {
	DB *pgxpool.Pool
}

type createPaymentIntentReq struct {
	AccountID   string `json:"account_id"`
	AmountCents int64  `json:"amount_cents"`
}

func (h *PaymentIntentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPaymentIntentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// if req.AmountCents > 10 {
	// 	WriteError(w, http.StatusBadRequest, "only 10 cents is allowed")
	// 	return
	// }

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid account_id")
		return
	}

	pi, err := repo.CreatePaymentIntent(
		r.Context(),
		h.DB,
		accountID,
		req.AmountCents,
	)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create payment_intent")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"id":           pi.ID.String(),
		"account_id":   pi.AccountID.String(),
		"amount_cents": pi.Amount,
		"status":       pi.Status,
	})
}

func (h *PaymentIntentsHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	intentID, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid intent id")
		return
	}

	err = repo.ConfirmPayment(
		r.Context(),
		h.DB,
		intentID,
		domain.DefaultPolicy(),
	)

	if err != nil {
		if err == repo.ErrInsufficientCredit {
			WriteError(w, http.StatusPaymentRequired, "insufficient credit")
			return
		} else if err == repo.ErrMoreThan10Cents {
			WriteError(w, http.StatusForbidden, "Refused payment over 10 cents and 10 dollars fined")
			return
		} else if err == repo.ErrAccountLocked {
			WriteError(w, http.StatusForbidden, "account locked")
			return
		}
		WriteError(w, http.StatusInternalServerError, "confirm failed")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
