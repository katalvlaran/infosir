package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"infosir/internal/model"
	"infosir/pkg/crypto"

	"github.com/stretchr/testify/assert"

	"infosir/cmd/handler"
	"infosir/internal/srv"
	"infosir/tests/mocks"
)

// If your main uses a mux or some router, replicate that logic here
// or if the orchestrator handler is public, you can call it directly.

func TestOrchestratorHandler_Integration(t *testing.T) {
	// Use mock binance and mock nats for a partial integration test
	scenarioList := []mocks.Scenario{
		{
			Pair: "BTCUSDT", Interval: "1m", Limit: 5,
			ReturnKlines: []model.Kline{
				{Time: crypto.MsToTime(1000_000)}, // ...
			},
			ReturnError: nil,
		},
		{
			Pair: "BTCUSDT", Interval: "1m", Limit: 10,
			// допустим хотим вернуть ошибку
			ReturnKlines: nil,
			ReturnError:  fmt.Errorf("simulate network fail"),
		},
	}
	mockBinance := mocks.NewMockBinanceClient(scenarioList)

	scenarios := []mocks.NatsPublishScenario{
		{Subject: "infosir_kline", ReturnError: nil}, // ok
		{Subject: "some_other_subject", ReturnError: fmt.Errorf("JS down")},
	}
	mockNats := mocks.NewMockNatsClient(scenarios)

	logger := testLogger()
	service := srv.NewInfoSirService(mockBinance, mockNats)

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
