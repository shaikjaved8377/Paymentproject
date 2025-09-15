package dataservice

import (
	"context"
	"database/sql"
	"time"
)


type PaymentRow struct {
	ID                     string
	OrderID                string
	AmountCents            int64
	Currency               string
	Status                 string
	AuthorizedAt           time.Time
	AuthorizationExpiresAt time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}


func InitDB(ctx context.Context, db *sql.DB) error {
	
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS payments (
  id                       VARCHAR(64) PRIMARY KEY,
  order_id                 VARCHAR(64) NOT NULL,
  amount_cents             BIGINT NOT NULL,
  currency                 CHAR(3) NOT NULL,
  status                   ENUM('authorized','captured','partially_refunded','refunded','failed') NOT NULL,
  authorized_at            DATETIME NOT NULL,
  authorization_expires_at DATETIME NOT NULL,
  captured_amount_cents    BIGINT NOT NULL DEFAULT 0,
  created_at               DATETIME NOT NULL,
  updated_at               DATETIME NOT NULL,
  INDEX (order_id),
  INDEX (status)
)`); err != nil {
		return err
	}

	// 2) idempotency (optional but handy)
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS idempotency_keys (
  k             VARCHAR(128) NOT NULL,
  endpoint      VARCHAR(64)  NOT NULL,
  response_json TEXT         NOT NULL,
  created_at    DATETIME     NOT NULL,
  PRIMARY KEY (k, endpoint)
)`); err != nil {
		return err
	}

	// 3) order sequence (for auto ORD-1, ORD-2, ...)
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS order_seq (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY
)`); err != nil {
		return err
	}

	return nil
}

// NextOrderSeq inserts a row into order_seq and returns the AUTO_INCREMENT id.
func NextOrderSeq(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx, `INSERT INTO order_seq () VALUES ()`)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertPayment saves one authorized payment row.
func InsertPayment(ctx context.Context, db *sql.DB, p PaymentRow) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO payments
  (id, order_id, amount_cents, currency, status, authorized_at, authorization_expires_at, captured_amount_cents, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		p.ID,
		p.OrderID,
		p.AmountCents,
		p.Currency,
		p.Status,
		p.AuthorizedAt,
		p.AuthorizationExpiresAt,
		p.CreatedAt,
		p.UpdatedAt,
	)
	return err
}


func GetIdempotency(ctx context.Context, db *sql.DB, key, endpoint string) (string, bool, error) {
	var resp string
	err := db.QueryRowContext(ctx, `SELECT response_json FROM idempotency_keys WHERE k=? AND endpoint=?`, key, endpoint).Scan(&resp)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return resp, true, nil
}

func SaveIdempotency(ctx context.Context, db *sql.DB, key, endpoint, responseJSON string, now time.Time) error {
	_, err := db.ExecContext(ctx, `
INSERT INTO idempotency_keys (k, endpoint, response_json, created_at)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE response_json=VALUES(response_json), created_at=VALUES(created_at)`,
		key, endpoint, responseJSON, now,
	)
	return err
}
