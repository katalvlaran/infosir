package nats

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gonats "github.com/nats-io/nats.go"
)

func TestRealBrokerExistingProfilesRejectEvictionAndRetryCaps(t *testing.T) {
	if os.Getenv("INFOSIR_NATS_INTEGRATION") != "1" {
		t.Skip("set INFOSIR_NATS_INTEGRATION=1")
	}
	binary, err := exec.LookPath("nats-server")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	dir := t.TempDir()
	log, err := os.Create(filepath.Join(dir, "broker.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	process := exec.Command(binary, "-js", "-a", "127.0.0.1", "-p", fmt.Sprint(port), "-sd", filepath.Join(dir, "store"))
	process.Stdout = log
	process.Stderr = log
	if err = process.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = process.Process.Kill(); _ = process.Wait() }()
	var nc *gonats.Conn
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nc, err = gonats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", port), gonats.Timeout(200*time.Millisecond))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.AddStream(&gonats.StreamConfig{Name: "profiles", Subjects: []string{"profiles"}, Storage: gonats.FileStorage, Retention: gonats.LimitsPolicy, Discard: gonats.DiscardNew, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err = validateStreamConfig(stream.Config, "profiles"); err != nil {
		t.Fatalf("actual server defaults rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*gonats.StreamConfig)
	}{
		{"age", func(c *gonats.StreamConfig) { c.MaxAge = time.Second; c.Duplicates = 500 * time.Millisecond }},
		{"discard_old", func(c *gonats.StreamConfig) { c.Discard = gonats.DiscardOld; c.MaxMsgs = 1 }},
		{"per_subject", func(c *gonats.StreamConfig) { c.MaxMsgsPerSubject = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := stream.Config
			tc.mutate(&c)
			actual, e := js.UpdateStream(&c)
			if e != nil {
				t.Fatal(e)
			}
			if validateStreamConfig(actual.Config, "profiles") == nil {
				t.Fatal("server accepted unsafe config and migration did not reject it")
			}
			if _, e = js.UpdateStream(&stream.Config); e != nil {
				t.Fatal(e)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		bad    bool
		mutate func(*gonats.ConsumerConfig)
	}{
		{"safe", false, func(*gonats.ConsumerConfig) {}},
		{"limited", true, func(c *gonats.ConsumerConfig) { c.MaxDeliver = 1 }},
		{"inactive", true, func(c *gonats.ConsumerConfig) { c.InactiveThreshold = time.Minute }},
		{"memory", true, func(c *gonats.ConsumerConfig) { c.MemoryStorage = true }},
		{"shortbackoff", true, func(c *gonats.ConsumerConfig) { c.BackOff = []time.Duration{time.Second} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := gonats.ConsumerConfig{Durable: tc.name, FilterSubject: "profiles", DeliverSubject: gonats.NewInbox(), AckPolicy: gonats.AckExplicitPolicy, AckWait: 30 * time.Second, MaxAckPending: 64}
			tc.mutate(&c)
			actual, e := js.AddConsumer("profiles", &c)
			if e != nil {
				t.Fatal(e)
			}
			e = validateConsumerConfig(actual.Config, "profiles", tc.name)
			if tc.bad && e == nil {
				t.Fatal("unsafe consumer accepted")
			}
			if !tc.bad && e != nil {
				t.Fatalf("server normalized defaults rejected: %v", e)
			}
		})
	}
}
