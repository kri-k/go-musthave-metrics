package agent_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kri-k/go-musthave-metrics/internal/agent"
	"github.com/stretchr/testify/require"
)

func TestReportSignature(t *testing.T) {
	for _, key := range []string{"", "secret"} {
		t.Run(key, func(t *testing.T) {
			var body []byte
			var header string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ = io.ReadAll(r.Body)
				header = r.Header.Get("HashSHA256")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			a := agent.NewAgentWithConfig(server.URL, 1, 1, key)
			a.Poll()
			a.Report()
			require.NotEmpty(t, body)
			if key == "" {
				require.Empty(t, header)
				return
			}
			h := hmac.New(sha256.New, []byte(key))
			h.Write(body)
			require.Equal(t, hex.EncodeToString(h.Sum(nil)), header)
		})
	}
}
