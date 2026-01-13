package outbox

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"gateway/internal/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	DB     *pgxpool.Pool
	Client *http.Client

	PollInterval time.Duration
	BatchSize    int

	WebhookSecret string
}

func NewWorker(db *pgxpool.Pool, secret string) *Worker {
	return &Worker{
		DB:            db,
		Client:        &http.Client{Timeout: 5 * time.Second},
		PollInterval:  500 * time.Millisecond,
		BatchSize:     20,
		WebhookSecret: secret,
	}
}

func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = w.DispatchOnce(ctx) // swallow errors; outbox is retry-driven
		}
	}
}

func (w *Worker) DispatchOnce(ctx context.Context) error {
	tx, err := w.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	evts, err := repo.ClaimPendingOutboxTx(ctx, tx, w.BatchSize)
	if err != nil {
		return err
	}
	if len(evts) == 0 {
		return tx.Commit(ctx)
	}

	for _, e := range evts {
		err := w.sendOne(ctx, e.TargetURL, e.EventID.String(), e.PayloadJSON)

		if err == nil {
			if err2 := repo.MarkOutboxSentTx(ctx, tx, e.ID); err2 != nil {
				return err2
			}
			continue
		}

		// simple backoff: 1s, 2s, 4s, ... max 60s
		attempt := e.AttemptCount + 1
		backoff := time.Second * time.Duration(1<<min(int(attempt-1), 6)) // cap ~64s
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}

		if err2 := repo.MarkOutboxFailedTx(ctx, tx, e.ID, attempt, err.Error(), backoff); err2 != nil {
			return err2
		}
	}

	return tx.Commit(ctx)
}

func (w *Worker) sendOne(ctx context.Context, url string, eventID string, payload []byte) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "1CENT-outbox/1.0")
	req.Header.Set("X-1CENT-Event-ID", eventID)

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("X-1CENT-Timestamp", ts)

	if w.WebhookSecret != "" {
		sig := sign(w.WebhookSecret, ts, payload)
		req.Header.Set("X-1CENT-Signature", sig)
	}

	resp, err := w.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sign(secret string, ts string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	// signature over: "<ts>.<raw_body>"
	m.Write([]byte(ts))
	m.Write([]byte("."))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}
