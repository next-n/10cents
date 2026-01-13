package httpx

import (
	"errors"
	"net/http"

	"gateway/internal/domain"
	"gateway/internal/repo"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *MerchantRequestsHandler) PayConfirmIntent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	intentID, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid payment intent id")
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), pgx.TxOptions{})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())

	// lock mapping row to get merchant_request_id
	mrID, err := repo.GetMerchantRequestIDByPaymentIntentForUpdate(r.Context(), tx, intentID)
	if err != nil {
		WriteError(w, http.StatusNotFound, "merchant pay intent not found")
		return
	}

	// lock merchant_request
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

	// confirm payment inside same tx (idempotent on payment_intents.status)
	err = repo.ConfirmPaymentTx(r.Context(), tx, intentID, domain.DefaultPolicy())
	if err != nil {
		if errors.Is(err, repo.ErrInsufficientCredit) || errors.Is(err, repo.ErrMoreThan10Cents) || errors.Is(err, repo.ErrAccountLocked) {
			_ = tx.Commit(r.Context())

			if errors.Is(err, repo.ErrAccountLocked) {
				WriteError(w, http.StatusForbidden, "account locked")
				return
			}
			if errors.Is(err, repo.ErrInsufficientCredit) {
				WriteError(w, http.StatusPaymentRequired, "insufficient credit")
				return
			}
			if errors.Is(err, repo.ErrMoreThan10Cents) {
				WriteError(w, http.StatusBadRequest, "amount > 10 cents")
				return
			}
		}

		WriteError(w, http.StatusInternalServerError, "payment confirm failed")
		return
	}

	// ✅ idempotency gate: only one confirm call can progress merchant request
	first, err := repo.TryMarkMerchantPayProgressedTx(r.Context(), tx, intentID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to mark merchant pay progressed")
		return
	}

	var paid, target int64
	var completedNow bool

	if first {
		paid, target, completedNow, err = repo.IncrementMerchantRequestProgress(r.Context(), tx, mrID, merchantPayAmountCents)
		if err != nil {
			if errors.Is(err, repo.ErrMerchantRequestNotPayable) {
				WriteError(w, http.StatusConflict, "merchant request not payable")
				return
			}
			WriteError(w, http.StatusInternalServerError, "failed to update merchant request")
			return
		}
	} else {
		// already progressed earlier -> just read current values
		mr2, err := repo.GetMerchantRequestByIDForUpdate(r.Context(), tx, mrID)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to reload merchant request")
			return
		}
		paid = mr2.PaidCents
		target = mr2.TargetCents
		completedNow = false
	}

	if err := tx.Commit(r.Context()); err != nil {
		WriteError(w, http.StatusInternalServerError, "transaction commit failed")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"status":                     "ok",
		"merchant_request_id":        mrID,
		"merchant_request_reference": mr.MerchantRequestReference,
		"payment_intent_id":          intentID.String(),
		"paid_cents":                 paid,
		"target_cents":               target,
		"completed_now":              completedNow,
		"idempotent_hit":             !first,
	})
}
