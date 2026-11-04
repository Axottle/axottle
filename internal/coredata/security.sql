ALTER TABLE admins ALTER COLUMN permissions SET DEFAULT '[]'::jsonb;

-- The chain starts when this migration is installed. Keep an off-host copy of
-- its head to detect truncation or a rewrite by a database administrator.
CREATE TABLE IF NOT EXISTS audit_integrity (
    seq BIGSERIAL PRIMARY KEY,
    audit_id BIGINT NOT NULL,
    operation TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    entry_hash TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS audit_integrity_entry ON audit_integrity(audit_id,seq DESC);

-- The initial chain entry bounds rows that predate integrity recording.
WITH baseline AS (
    SELECT COALESCE(MAX(id),0) AS cutoff, encode(sha256(convert_to('','UTF8')),'hex') AS digest FROM audit_log
)
INSERT INTO audit_integrity(audit_id,operation,payload_hash,previous_hash,entry_hash)
SELECT cutoff,'START',digest,'',encode(sha256(convert_to('START:'||cutoff::text||':'||digest||':','UTF8')),'hex')
FROM baseline WHERE NOT EXISTS (SELECT 1 FROM audit_integrity);

CREATE OR REPLACE FUNCTION record_audit_integrity() RETURNS trigger AS $$
DECLARE
    payload JSONB;
    previous TEXT;
    digest TEXT;
    row_id BIGINT;
BEGIN
    IF TG_OP = 'DELETE' THEN payload := to_jsonb(OLD); row_id := OLD.id;
    ELSE payload := to_jsonb(NEW); row_id := NEW.id; END IF;
    PERFORM pg_advisory_xact_lock(1616531332);
    SELECT entry_hash INTO previous FROM audit_integrity ORDER BY seq DESC LIMIT 1;
    previous := COALESCE(previous,'');
    digest := encode(sha256(convert_to(payload::text,'UTF8')),'hex');
    INSERT INTO audit_integrity(audit_id,operation,payload_hash,previous_hash,entry_hash)
    VALUES(row_id,TG_OP,digest,previous,
      encode(sha256(convert_to(TG_OP||':'||row_id::text||':'||digest||':'||previous,'UTF8')),'hex'));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql SET timezone = 'UTC';
DROP TRIGGER IF EXISTS audit_integrity_record ON audit_log;
CREATE TRIGGER audit_integrity_record AFTER INSERT OR UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION record_audit_integrity();

CREATE OR REPLACE FUNCTION protect_audit_integrity() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit integrity history is append-only';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS audit_integrity_protect ON audit_integrity;
CREATE TRIGGER audit_integrity_protect BEFORE UPDATE OR DELETE OR TRUNCATE ON audit_integrity
    FOR EACH STATEMENT EXECUTE FUNCTION protect_audit_integrity();
