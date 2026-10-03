package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/signature"
	"github.com/stretchr/testify/require"
)

func TestHashSHA256(t *testing.T) {
	for _, tc := range []struct {
		name, key, hash string
		status          int
	}{
		{"valid", "secret", signature.Sign([]byte("request"), "secret"), 201},
		{"wrong key", "secret", signature.Sign([]byte("request"), "other"), 400},
		{"changed body", "secret", signature.Sign([]byte("other"), "secret"), 400},
		{"missing", "secret", "", 400},
		{"malformed", "secret", "xyz", 400},
		{"disabled", "", "xyz", 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := HashSHA256(tc.key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Equal(t, "request", string(body))
				w.Header().Set("X-Test", "preserved")
				w.WriteHeader(201)
				_, _ = w.Write([]byte("first"))
				_, _ = w.Write([]byte("second"))
			}))
			req := httptest.NewRequest("POST", "/", bytes.NewBufferString("request"))
			req.Header.Set(signature.Header, tc.hash)
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, req)
			require.Equal(t, tc.status, result.Code)
			require.Equal(t, tc.status == 201, called)
			if tc.key != "" {
				require.True(t, signature.Verify(result.Body.Bytes(), tc.key, result.Header().Get(signature.Header)))
			} else {
				require.Empty(t, result.Header().Get(signature.Header))
			}
			if called {
				require.Equal(t, "firstsecond", result.Body.String())
				require.Equal(t, "preserved", result.Header().Get("X-Test"))
			}
		})
	}
}

func TestHashSHA256Gzip(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, err := zw.Write([]byte(`{"value":42}`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	req := httptest.NewRequest("POST", "/", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set(signature.Header, signature.Sign(compressed.Bytes(), "secret"))
	result := httptest.NewRecorder()
	HashSHA256("secret")(Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, `{"value":42}`, string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))).ServeHTTP(result, req)
	require.Equal(t, http.StatusOK, result.Code)
	require.Equal(t, "gzip", result.Header().Get("Content-Encoding"))
	require.True(t, signature.Verify(result.Body.Bytes(), "secret", result.Header().Get(signature.Header)))
	zr, err := gzip.NewReader(result.Body)
	require.NoError(t, err)
	defer zr.Close()
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	require.Equal(t, `{"value":42}`, string(body))
}
