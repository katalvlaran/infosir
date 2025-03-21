package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"infosir/internal/model"

	"go.uber.org/zap"
)

// OrchestratorRequest описывает входные параметры запроса от оркестратора
type OrchestratorRequest struct {
	Pair   string `json:"pair"`
	Time   string `json:"time"`
	Klines int64  `json:"klines"`
}

// InfoSirService — временный интерфейс-заглушка, чтобы показать,
// как мы будем вызывать методы из service.go (Шаг 6).
// Когда реализуем реальный сервис, нужно будет подключить его сюда.
type InfoSirService interface {
	GetKlines(reqCtx context.Context, pair string, interval string, limit int64) ([]model.Kline, error)
	PublishKlines(reqCtx context.Context, klines []model.Kline) error
}

// OrchestratorHandler возвращает http.HandlerFunc, который обрабатывает запрос от оркестратора.
// Когда появится реальный InfoSirService, заменим аргумент service на настоящий.
func OrchestratorHandler(service InfoSirService, logger *zap.Logger) http.HandlerFunc {
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
			if err := service.PublishKlines(r.Context(), klines); err != nil {
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
