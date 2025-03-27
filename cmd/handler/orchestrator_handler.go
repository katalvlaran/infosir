package handler

import (
	"encoding/json"
	"net/http"

	"infosir/internal/srv"

	"go.uber.org/zap"
)

// OrchestratorRequest описывает входные параметры запроса от оркестратора
type OrchestratorRequest struct {
	Pair   string `json:"pair"`
	Time   string `json:"time"`
	Klines int64  `json:"klines"`
}

// OrchestratorRequest holds the request payload from an orchestrator
// for fetching & publishing klines.
func OrchestratorHandler(service srv.InfoSirService, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		var req OrchestratorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("Failed to decode orchestrator request", zap.Error(err))
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Простейшая валидация входных данных
		if req.Pair == "" || req.Klines <= 0 {
			logger.Error("Invalid arguments in orchestrator request",
				zap.String("pair", req.Pair),
				zap.Int64("klines", req.Klines))
			http.Error(w, "Invalid arguments", http.StatusBadRequest)
			return
		}

		// Если нет реального сервиса — временно пропускаем
		if service != nil {
			// Предположим, что service.GetKlines() вернёт набор свечей (interface{} пока что)
			klines, err := service.GetKlines(r.Context(), req.Pair, req.Time, req.Klines)
			if err != nil {
				logger.Error("Failed to get klines", zap.Error(err))
				http.Error(w, "Failed to fetch klines", http.StatusInternalServerError)
				return
			}

			// Затем публикуем результат в NATS
			if err := service.PublishKlinesJS(r.Context(), klines); err != nil {
				logger.Error("Failed to publish klines", zap.Error(err))
				http.Error(w, "Failed to publish klines", http.StatusInternalServerError)
				return
			}
		} else {
			logger.Warn("OrchestratorHandler called without a real InfoSirService implementation")
		}

		// Всё прошло успешно — отправляем статус 200
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]string{"status": "ok"}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
