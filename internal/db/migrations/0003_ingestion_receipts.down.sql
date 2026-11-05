-- Destructive rollback: export receipts/quarantine before using in an approved recovery.
DROP TABLE infosir_quarantine;
DROP TABLE infosir_consumer_inbox;
DROP TABLE infosir_storage_profile;
