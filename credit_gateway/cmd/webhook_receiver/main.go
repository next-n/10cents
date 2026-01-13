package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

func verify(secret, ts, sig string, body []byte) bool {
	if secret == "" || ts == "" || sig == "" {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(ts))
	m.Write([]byte("."))
	m.Write(body)
	expected := hex.EncodeToString(m.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

type Deduper struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func NewDeduper() *Deduper {
	return &Deduper{seen: make(map[string]struct{})}
}

func (d *Deduper) Seen(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[id]; ok {
		return true
	}
	d.seen[id] = struct{}{}
	return false
}

func main() {
	secret := os.Getenv("WEBHOOK_SECRET")
	deduper := NewDeduper()

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		eventID := r.Header.Get("X-1CENT-Event-ID")
		ts := r.Header.Get("X-1CENT-Timestamp")
		sig := r.Header.Get("X-1CENT-Signature")

		// 1) replay protection (±5 minutes)
		tsInt, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			http.Error(w, "bad timestamp", http.StatusBadRequest)
			return
		}
		now := time.Now().Unix()
		if tsInt < now-300 || tsInt > now+300 {
			http.Error(w, "stale timestamp", http.StatusUnauthorized)
			return
		}

		// 2) signature verification
		ok := verify(secret, ts, sig, body)
		if !ok {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		// 3) idempotency (dedupe by event_id)
		if eventID != "" && deduper.Seen(eventID) {
			log.Printf("DUPLICATE EVENT event_id=%s -> ack 200", eventID)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "duplicate ok")
			return
		}

		log.Printf("WEBHOOK OK event_id=%s ts=%s sig=%s\n%s\n", eventID, ts, sig, string(body))

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	log.Println("webhook receiver listening on :8090")
	log.Fatal(http.ListenAndServe(":8090", nil))
}
