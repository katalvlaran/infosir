package nats

import (
	"testing"
	"time"

	gonats "github.com/nats-io/nats.go"
)

func TestExistingStreamCannotEvictUnacknowledgedMessages(t *testing.T) {
	safe := gonats.StreamConfig{Storage: gonats.FileStorage, Subjects: []string{"bars"}, Retention: gonats.LimitsPolicy, Discard: gonats.DiscardNew, MaxMsgs: -1, MaxMsgsPerSubject: -1}
	if err := validateStreamConfig(safe, "bars"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*gonats.StreamConfig)
	}{
		{"age_expiry", func(c *gonats.StreamConfig) { c.MaxAge = time.Second }},
		{"discard_old_quota", func(c *gonats.StreamConfig) { c.Discard = gonats.DiscardOld; c.MaxMsgs = 1 }},
		{"per_subject_eviction", func(c *gonats.StreamConfig) { c.MaxMsgsPerSubject = 1 }},
		{"interest_retention", func(c *gonats.StreamConfig) { c.Retention = gonats.InterestPolicy }},
		{"workqueue_retention", func(c *gonats.StreamConfig) { c.Retention = gonats.WorkQueuePolicy }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := safe
			tc.mutate(&c)
			if validateStreamConfig(c, "bars") == nil {
				t.Fatal("unsafe existing stream accepted")
			}
		})
	}
	// Positive global quotas are safe with DiscardNew: the publisher gets an error.
	safe.MaxMsgs = 1
	safe.MaxMsgsPerSubject = 0
	if err := validateStreamConfig(safe, "bars"); err != nil {
		t.Fatal(err)
	}
}

func TestExistingConsumerKeepsDurableUnlimitedRetry(t *testing.T) {
	safe := gonats.ConsumerConfig{Durable: "writer", FilterSubject: "bars", DeliverSubject: "delivery", AckPolicy: gonats.AckExplicitPolicy, AckWait: 30 * time.Second, MaxAckPending: 64, MaxDeliver: -1}
	for _, n := range []int{-1, 0} {
		c := safe
		c.MaxDeliver = n
		if err := validateConsumerConfig(c, "bars", "writer"); err != nil {
			t.Fatalf("unlimited %d: %v", n, err)
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*gonats.ConsumerConfig)
	}{
		{"finite_deliveries", func(c *gonats.ConsumerConfig) { c.MaxDeliver = 1 }},
		{"inactive_deletion", func(c *gonats.ConsumerConfig) { c.InactiveThreshold = time.Minute }},
		{"memory_state", func(c *gonats.ConsumerConfig) { c.MemoryStorage = true }},
		{"backoff_overrides_ackwait", func(c *gonats.ConsumerConfig) { c.BackOff = []time.Duration{time.Second, 30 * time.Second} }},
		{"later_short_backoff", func(c *gonats.ConsumerConfig) { c.BackOff = []time.Duration{30 * time.Second, time.Second} }},
		{"ephemeral", func(c *gonats.ConsumerConfig) { c.Durable = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := safe
			tc.mutate(&c)
			if validateConsumerConfig(c, "bars", "writer") == nil {
				t.Fatal("unsafe existing consumer accepted")
			}
		})
	}
	safe.BackOff = []time.Duration{30 * time.Second, time.Minute}
	if err := validateConsumerConfig(safe, "bars", "writer"); err != nil {
		t.Fatal(err)
	}
}
