package model

type RefundReq struct {
	PaymentID int64 `json:"payment_id"`
}

type RefundResponse struct {
	RefundID            int64   `json:"refund_id"`
	PaymentID           int64   `json:"payment_id"`
	RefundedAmountTotal float64 `json:"refunded_amount_total"`
	Status              string  `json:"status"` // "refunded"
}
