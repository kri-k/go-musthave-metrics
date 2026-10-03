package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/kri-k/go-musthave-metrics/internal/signature"
)

func HashSHA256(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := &hashResponseWriter{header: make(http.Header)}
			body, err := io.ReadAll(r.Body)
			if err != nil || !signature.Verify(body, key, r.Header.Get(signature.Header)) {
				http.Error(response, "invalid request signature", http.StatusBadRequest)
			} else {
				r.Body = io.NopCloser(bytes.NewReader(body))
				next.ServeHTTP(response, r)
			}
			for name, values := range response.Header() {
				w.Header()[name] = values
			}
			w.Header().Set(signature.Header, signature.Sign(response.body.Bytes(), key))
			status := response.status
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			_, _ = w.Write(response.body.Bytes())
		})
	}
}

type hashResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *hashResponseWriter) Header() http.Header { return w.header }

func (w *hashResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *hashResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		if w.header.Get("Content-Type") == "" {
			w.header.Set("Content-Type", http.DetectContentType(body))
		}
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(body)
}
