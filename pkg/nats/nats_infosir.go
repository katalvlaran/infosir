package natsinfosir

import (
	"github.com/nats-io/nats.go"
)

// NatsClient определяет методы для отправки сообщений в NATS.
// При необходимости можно добавить методы подписки (Subscribe).
type NatsClient interface {
	Publish(subject string, data []byte) error
}

// natsClientImpl — реализация NatsClient
type natsClientImpl struct {
	conn *nats.Conn
}

// NewNatsClient — конструктор, принимает *nats.Conn (инициализированное в main.go)
func NewNatsClient(conn *nats.Conn) NatsClient {
	return &natsClientImpl{conn: conn}
}

// Publish отправляет data в указанную тему (subject) NATS.
// При ошибке возвращает error.
func (n *natsClientImpl) Publish(subject string, data []byte) error {
	return n.conn.Publish(subject, data)
}
