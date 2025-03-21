package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"infosir/cmd/handler"
	"infosir/internal/srv"
	"infosir/tests/mocks"
)

// If your main uses a mux or some router, replicate that logic here
// or if the orchestrator handler is public, you can call it directly.

func TestOrchestratorHandler_Integration(t *testing.T) {
	// Use mock binance and mock nats for a partial integration test
	mockBinance := mocks.NewMockBinanceClient()
	mockNats := mocks.NewMockNatsClient()

	logger := testLogger()
	service := srv.NewInfoSirService(logger, testConfig, mockBinance, mockNats)

	// Build a handler with service
	handlerFn := handler.OrchestratorHandler(service, logger)

	// Prepare request
	body := []byte(`{"pair":"BTCUSDT","time":"1m","klines":3}`)
	req := httptest.NewRequest(http.MethodPost, "/orchestrator/fetch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// record response
	w := httptest.NewRecorder()
	handlerFn(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// check body
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, "ok", result["status"], "Expected JSON {status: ok}")
}

// If you prefer a more thorough approach, spin up an actual server and call it via net/http.
