package model

import "time"

type AuthorizeRequest struct {
	OrderID            string `json:"order_id"`
	AmountCents        int64  `json:"amount_cents"`
	Currency           string `json:"currency"`
	PaymentMethodToken string `json:"payment_method_token"`
	IdempotencyKey     string `json:"idempotency_key"`
}

type AuthorizeResponse struct {
	PaymentID              string    `json:"payment_id"`
	Status                 string    `json:"status"`
	AuthorizationExpiresAt time.Time `json:"authorization_expires_at"`
}

type Payment struct {
	ID                     string
	OrderID                string
	AmountCents            int64
	Currency               string
	Status                 string
	AuthorizedAt           time.Time
	AuthorizationExpiresAt time.Time
	CapturedAmountCents    int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type RefundReq struct {
	PaymentID string `json:"payment_id"`
}

type RefundResponse struct {
	RefundID            int64  `json:"refund_id"`
	PaymentID           string `json:"payment_id"`
	RefundedAmountCents int64  `json:"refunded_amount_cents"`
	Status              string `json:"status"`
}
