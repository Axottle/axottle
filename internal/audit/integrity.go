package audit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
)

type IntegrityHead struct {
	Sequence int64  `json:"sequence"`
	Hash     string `json:"hash"`
}

// VerifyIntegrity validates the chain and the latest recorded state of rows.
// It covers events since installation of the integrity migration. An off-host
// checkpoint is needed to detect replacement of the entire database history.
func VerifyIntegrity(ctx context.Context, db *sql.DB) (IntegrityHead, error) {
	var head IntegrityHead
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return head, err
	}
	defer tx.Rollback()
	// PostgreSQL JSON renders timestamps in the session timezone. Match the trigger.
	if _, err := tx.ExecContext(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
		return head, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT seq,audit_id,operation,payload_hash,previous_hash,entry_hash FROM audit_integrity ORDER BY seq`)
	if err != nil {
		return head, err
	}
	var cutoff int64
	for rows.Next() {
		var seq, id int64
		var op, payload, previous, digest string
		if err := rows.Scan(&seq, &id, &op, &payload, &previous, &digest); err != nil {
			rows.Close()
			return head, err
		}
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s:%s", op, id, payload, previous)))
		if previous != head.Hash || hex.EncodeToString(sum[:]) != digest {
			rows.Close()
			return head, fmt.Errorf("audit integrity mismatch at sequence %d", seq)
		}
		if head.Sequence == 0 {
			if op != "START" {
				rows.Close()
				return head, fmt.Errorf("audit integrity baseline is missing")
			}
			cutoff = id
		} else if op != "INSERT" && op != "UPDATE" && op != "DELETE" {
			rows.Close()
			return head, fmt.Errorf("invalid audit operation at sequence %d", seq)
		}
		head = IntegrityHead{Sequence: seq, Hash: digest}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return head, err
	}
	if head.Sequence == 0 {
		return head, fmt.Errorf("audit integrity baseline is missing")
	}
	var mismatch bool
	err = tx.QueryRowContext(ctx, `WITH latest AS (SELECT DISTINCT ON(audit_id) audit_id,operation,payload_hash FROM audit_integrity WHERE operation<>'START' ORDER BY audit_id,seq DESC)
 SELECT EXISTS(SELECT 1 FROM latest l LEFT JOIN audit_log a ON a.id=l.audit_id
 WHERE (l.operation='DELETE' AND a.id IS NOT NULL) OR (l.operation<>'DELETE' AND (a.id IS NULL OR encode(sha256(convert_to(to_jsonb(a)::text,'UTF8')),'hex')<>l.payload_hash)))
 OR EXISTS(SELECT 1 FROM audit_log a WHERE a.id>$1 AND NOT EXISTS(SELECT 1 FROM latest l WHERE l.audit_id=a.id))`, cutoff).Scan(&mismatch)
	if err != nil {
		return head, err
	}
	if mismatch {
		return head, fmt.Errorf("audit rows differ from recorded history")
	}
	return head, tx.Commit()
}
