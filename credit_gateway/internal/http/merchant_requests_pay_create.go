package httpx

import (
	"net/http"
	"strconv"

	"gateway/internal/repo"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const merchantPayAmountCents = int64(10)

func (h *MerchantRequestsHandler) PayCreateIntent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	mrID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || mrID <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid merchant request id")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	mr, err := repo.GetMerchantRequestByIDForUpdate(r.Context(), tx, mrID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "merchant request not found")
		return
	}

	if mr.Status != "pending" {
		WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "already_closed",
			"paid_cents":   mr.PaidCents,
			"target_cents": mr.TargetCents,
		})
		return
	}

	// if already completed by numbers (safety)
	if mr.PaidCents >= mr.TargetCents {
		WriteJSON(w, http.StatusOK, map[string]any{
			"status":       "already_fulfilled",
			"paid_cents":   mr.PaidCents,
			"target_cents": mr.TargetCents,
		})
		return
	}

	accountID, err := uuid.Parse(mr.PayerAccountID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "invalid payer_account_id")
		return
	}

	pi, err := repo.CreateMerchantPayIntentTx(
		r.Context(),
		tx,
		mrID,
		accountID,
		merchantPayAmountCents,
	)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create merchant pay intent")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		WriteError(w, http.StatusInternalServerError, "transaction commit failed")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"status":              "created",
		"merchant_request_id": mrID,
		"payment_intent_id":   pi.ID.String(),
		"amount_cents":        pi.Amount,
		"intent_status":       pi.Status, // should be "pending"
	})
}
