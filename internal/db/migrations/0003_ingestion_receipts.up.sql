-- Additive operational hardening for the single-profile legacy schema.
-- This does not claim to implement the full revisioned v2 market journal.
CREATE TABLE infosir_storage_profile (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 market text NOT NULL CHECK(market IN ('spot','usdm')),
 interval_code text NOT NULL,
 bound_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE infosir_consumer_inbox (
 consumer_id text NOT NULL, event_id text NOT NULL,
 payload_hash text NOT NULL CHECK(payload_hash ~ '^[0-9a-f]{64}$'),
 processed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(consumer_id,event_id)
);
CREATE TABLE infosir_quarantine (
 quarantine_id text PRIMARY KEY,
 consumer_id text NOT NULL, event_id text NOT NULL,
 reason_code text NOT NULL,
 payload bytea NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
