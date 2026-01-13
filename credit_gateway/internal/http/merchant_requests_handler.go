package httpx

import (
	"encoding/json"
	"net/http"
	"strconv"

	"gateway/internal/repo"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MerchantRequestsHandler struct {
	DB *pgxpool.Pool
}

type createMerchantRequestReq struct {
	MerchantID              string  `json:"merchant_id"`
	MerchantRequestRefrence *string `json:"merchant_request_reference"`
	TargetCents             int64   `json:"target_cents"`
	WebhookURL              *string `json:"webhook_url"`
	PayerAccountID          string  `json:"payer_account_id"`
}

func (h *MerchantRequestsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createMerchantRequestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.MerchantID == "" {
		WriteError(w, http.StatusBadRequest, "missing merchant_id")
		return
	}
	if req.PayerAccountID == "" {
		WriteError(w, http.StatusBadRequest, "missing payer_account_id")
		return
	}
	if req.TargetCents <= 0 {
		WriteError(w, http.StatusBadRequest, "target_cents must be > 0")
		return
	}

	mr, err := repo.CreateMerchantRequest(
		r.Context(),
		h.DB,
		req.MerchantID,
		req.MerchantRequestRefrence,
		req.PayerAccountID,
		req.TargetCents,
		req.WebhookURL,
	)
	if err != nil {
		if err == repo.ErrDuplicateMerchantRequest {
			WriteError(w, http.StatusConflict, "duplicate merchant_request_reference")
			return
		}
		// if payer_account_id FK fails, Postgres returns 23503
		WriteError(w, http.StatusInternalServerError, "failed to create merchant request")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"id":                         mr.ID,
		"merchant_id":                mr.MerchantID,
		"merchant_request_reference": mr.MerchantRequestReference,
		"payer_account_id":           mr.PayerAccountID,
		"target_cents":               mr.TargetCents,
		"paid_cents":                 mr.PaidCents,
		"status":                     mr.Status,
		"webhook_url":                mr.WebhookURL,
	})
}

func (h *MerchantRequestsHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		WriteError(w, http.StatusBadRequest, "missing merchant request id")
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid merchant request id")
		return
	}

	mr, err := repo.GetMerchantRequestByID(r.Context(), h.DB, id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "merchant request not found")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"id":                         mr.ID,
		"merchant_id":                mr.MerchantID,
		"merchant_request_reference": mr.MerchantRequestReference,
		"target_cents":               mr.TargetCents,
		"paid_cents":                 mr.PaidCents,
		"status":                     mr.Status,
		"webhook_url":                mr.WebhookURL,
		"completed_at":               mr.CompletedAt,
		"created_at":                 mr.CreatedAt,
		"updated_at":                 mr.UpdatedAt,
	})
}
