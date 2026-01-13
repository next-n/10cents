package httpx

import (
	"context"

	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// middleware (keep it sane)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)

	// health
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			WriteError(w, http.StatusServiceUnavailable, "db not ready")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/v1", func(r chi.Router) {
		h := &AccountsHandler{DB: db}
		r.Get("/accounts/{id}", h.GetByID)

		pi := &PaymentIntentsHandler{DB: db}
		r.Post("/payment_intents", pi.Create)
		r.Post("/payment_intents/{id}/confirm", pi.Confirm)

		mrh := &MerchantRequestsHandler{DB: db}
		r.Post("/merchant_requests", mrh.Create)
		r.Get("/merchant_requests/{id}", mrh.GetByID)
		// r.Post("/merchant_requests/{id}/pay", mrh.Pay)

		r.Post("/merchant_requests/{id}/pay", mrh.PayCreateIntent)

		// confirm merchant-payment intent
		r.Post("/merchant_requests/payment_intents/{id}/confirm", mrh.PayConfirmIntent)

	})
	return r
}
