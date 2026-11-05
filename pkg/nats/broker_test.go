package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"infosir/internal/models"

	gonats "github.com/nats-io/nats.go"
)

type retryOnceStore struct{ attempts atomic.Int32 }

func (s *retryOnceStore) CommitMessage(context.Context, string, string, []byte, []models.Kline) error {
	if s.attempts.Add(1) == 1 {
		return fmt.Errorf("injected unavailable database")
	}
	return nil
}
func (s *retryOnceStore) Quarantine(context.Context, string, string, string, []byte) error {
	return fmt.Errorf("unexpected quarantine")
}
func TestRealBrokerRedeliveryAndDurableDrain(t *testing.T) {
	if os.Getenv("INFOSIR_NATS_INTEGRATION") != "1" {
		t.Skip("set INFOSIR_NATS_INTEGRATION=1 with local nats-server")
	}
	binary, e := exec.LookPath("nats-server")
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	dir := t.TempDir()
	log, e := os.Create(filepath.Join(dir, "server.log"))
	if e != nil {
		t.Fatal(e)
	}
	defer log.Close()
	process := exec.Command(binary, "-js", "-a", "127.0.0.1", "-p", fmt.Sprint(port), "-sd", filepath.Join(dir, "store"))
	process.Stdout = log
	process.Stderr = log
	if e = process.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = process.Process.Kill(); _ = process.Wait() }()
	var nc *gonats.Conn
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nc, e = gonats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port), gonats.Timeout(200*time.Millisecond))
		if e == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if e != nil {
		t.Fatal(e)
	}
	defer nc.Close()
	js, e := nc.JetStream()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = js.AddStream(&gonats.StreamConfig{Name: "bars", Subjects: []string{"bars"}, Storage: gonats.FileStorage}); e != nil {
		t.Fatal(e)
	}
	if _, e = js.AddConsumer("bars", &gonats.ConsumerConfig{Durable: "writer", DeliverSubject: gonats.NewInbox(), FilterSubject: "bars", AckPolicy: gonats.AckExplicitPolicy, AckWait: 2 * time.Second, MaxAckPending: 4}); e != nil {
		t.Fatal(e)
	}
	now := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	profile := ConsumerProfile{Consumer: "writer", Market: "spot", Interval: "1m", Clock: func() time.Time { return now }}
	store := &retryOnceStore{}
	acked := make(chan error, 4)
	sub, e := js.Subscribe("bars", func(m *gonats.Msg) {
		err := ProcessDelivery(context.Background(), brokerDelivery{m}, store, profile)
		if store.attempts.Load() >= 2 {
			acked <- err
		}
	}, gonats.Bind("bars", "writer"), gonats.ManualAck())
	if e != nil {
		t.Fatal(e)
	}
	k := models.Kline{Time: now.Add(-time.Minute), Symbol: "BTCUSDT", OpenPrice: 1, HighPrice: 1, LowPrice: 1, ClosePrice: 1}
	data, _ := json.Marshal([]models.Kline{k})
	msg := gonats.NewMsg("bars")
	msg.Data = data
	msg.Header.Set(gonats.MsgIdHdr, "stable")
	msg.Header.Set("Infosir-Schema", "1")
	msg.Header.Set("Infosir-Market", "spot")
	msg.Header.Set("Infosir-Interval", "1m")
	if _, e = js.PublishMsg(msg); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-acked:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("redelivery missing")
	}
	if store.attempts.Load() != 2 {
		t.Fatalf("attempts %d", store.attempts.Load())
	}
	info, e := js.ConsumerInfo("bars", "writer")
	if e != nil || info.NumAckPending != 0 {
		t.Fatalf("pending after commit %v %v", info, e)
	}
	if e = sub.Drain(); e != nil {
		t.Fatal(e)
	}
	for sub.IsValid() && time.Now().Before(deadline.Add(8*time.Second)) {
		time.Sleep(10 * time.Millisecond)
	}
	if sub.IsValid() {
		t.Fatal("subscription drain timeout")
	}
	if _, e = js.ConsumerInfo("bars", "writer"); e != nil {
		t.Fatalf("drain deleted durable: %v", e)
	}
}
