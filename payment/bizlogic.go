package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"

	"payment/dataservice"
	"payment/model"
)

type Service struct {
	DB           *sql.DB
	Producer     sarama.SyncProducer
	TopicAuthzed string
}

func NewService(db *sql.DB, producer sarama.SyncProducer, topic string) *Service {
	return &Service{DB: db, Producer: producer, TopicAuthzed: topic}
}

func (s *Service) Authorize(ctx context.Context, req model.AuthorizeRequest) (model.AuthorizeResponse, error) {
	if req.AmountCents <= 0 ||
		len(req.Currency) != 3 ||
		strings.TrimSpace(req.PaymentMethodToken) == "" {
		return model.AuthorizeResponse{}, errors.New("invalid request fields")
	}

	const endpoint = "authorize"
	if req.IdempotencyKey != "" {
		if cached, ok, err := dataservice.GetIdempotency(ctx, s.DB, req.IdempotencyKey, endpoint); err == nil && ok {
			var resp model.AuthorizeResponse
			_ = json.Unmarshal([]byte(cached), &resp)
			return resp, nil
		}
	}

	now := time.Now().UTC()
	expires := now.Add(7 * 24 * time.Hour)

	// Auto-generate
	orderID := strings.TrimSpace(req.OrderID)
	if orderID == "" {
		n, err := dataservice.NextOrderSeq(ctx, s.DB)
		if err != nil {
			return model.AuthorizeResponse{}, err
		}
		orderID = fmt.Sprintf("ORD-%d", n)
	}

	paymentID := "pay_" + uuid.NewString()
	row := dataservice.PaymentRow{
		ID:                     paymentID,
		OrderID:                orderID,
		AmountCents:            req.AmountCents,
		Currency:               strings.ToUpper(req.Currency),
		Status:                 "authorized",
		AuthorizedAt:           now,
		AuthorizationExpiresAt: expires,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := dataservice.InsertPayment(ctx, s.DB, row); err != nil {
		return model.AuthorizeResponse{}, err
	}

	ev := struct {
		Type       string `json:"type"`
		PaymentID  string `json:"payment_id"`
		OrderID    string `json:"order_id"`
		Amount     int64  `json:"amount_cents"`
		Currency   string `json:"currency"`
		Status     string `json:"status"`
		OccurredAt string `json:"occurred_at"`
	}{
		Type:       "payment.authorized",
		PaymentID:  paymentID,
		OrderID:    orderID,
		Amount:     req.AmountCents,
		Currency:   strings.ToUpper(req.Currency),
		Status:     "authorized",
		OccurredAt: now.Format(time.RFC3339),
	}
	b, _ := json.Marshal(ev)

	msg := &sarama.ProducerMessage{
		Topic: s.TopicAuthzed,
		Value: sarama.ByteEncoder(b),
	}
	if s.Producer != nil {
		_, _, _ = s.Producer.SendMessage(msg)
	}

	resp := model.AuthorizeResponse{
		PaymentID:              paymentID,
		Status:                 "authorized",
		AuthorizationExpiresAt: expires,
	}

	if req.IdempotencyKey != "" {
		j, _ := json.Marshal(resp)
		_ = dataservice.SaveIdempotency(ctx, s.DB, req.IdempotencyKey, endpoint, string(j), now)
	}

	return resp, nil
}

type IbizLogic interface {
	CapturePaymentLogic(paymentID string, amount int64) (model.CaptureResponse, error)
	RefundPaymentLogic(paymentID string) (model.RefundResponse, error)
}

type bizlogic struct {
	DB       *sql.DB
	Producer sarama.SyncProducer
}

func NewBizLogic(db *sql.DB, producer sarama.SyncProducer) *bizlogic {
	return &bizlogic{DB: db, Producer: producer}
}

func (bl *bizlogic) CapturePaymentLogic(paymentID string, amount int64) (model.CaptureResponse, error) {
	var out model.CaptureResponse

	// Update captured amount and status
	res, err := bl.DB.Exec(`
		UPDATE payments
		SET status = 'captured',
		    captured_amount_cents = ?,
		    updated_at = NOW()
		WHERE id = ? AND status = 'authorized'
	`, amount, paymentID)
	if err != nil {
		return out, err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return out, fmt.Errorf("payment not eligible for capture")
	}

	out = model.CaptureResponse{
		PaymentID:           paymentID,
		CapturedAmountCents: amount,
		Status:              "captured",
	}

	// Kafka event
	if bl.Producer != nil {
		ev := struct {
			Type       string `json:"type"`
			PaymentID  string `json:"payment_id"`
			Amount     int64  `json:"amount_cents"`
			Status     string `json:"status"`
			OccurredAt string `json:"occurred_at"`
		}{
			Type:       "payment.captured",
			PaymentID:  paymentID,
			Amount:     amount,
			Status:     "captured",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		}
		b, _ := json.Marshal(ev)
		_, _, _ = bl.Producer.SendMessage(&sarama.ProducerMessage{
			Topic: "payments_captured",
			Value: sarama.ByteEncoder(b),
		})
	}

	return out, nil
}

func (bl *bizlogic) RefundPaymentLogic(paymentID string) (model.RefundResponse, error) {
	var out model.RefundResponse

	res, err := bl.DB.Exec(`
        UPDATE payments
        SET status = 'refunded', updated_at = NOW()
        WHERE id = ? AND status = 'captured'
    `, paymentID)
	if err != nil {
		return out, err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {

		return out, fmt.Errorf("payment not eligible for refund")
	}

	// 2) Insert
	res2, err := bl.DB.Exec(`
        INSERT INTO refunds (payment_id, amount_cents, created_at)
        SELECT id, captured_amount_cents, NOW()
        FROM payments
        WHERE id = ?
    `, paymentID)
	if err != nil {
		return out, err
	}
	refundID, _ := res2.LastInsertId()

	// 3) Read
	var amount int64
	if err := bl.DB.QueryRow(`SELECT captured_amount_cents FROM payments WHERE id = ?`, paymentID).
		Scan(&amount); err != nil {
		return out, err
	}

	out = model.RefundResponse{
		RefundID:            refundID,
		PaymentID:           paymentID,
		RefundedAmountCents: amount,
		Status:              "refunded",
	}

	if bl.Producer != nil {
		ev := struct {
			Type       string `json:"type"`
			PaymentID  string `json:"payment_id"`
			Amount     int64  `json:"amount_cents"`
			Status     string `json:"status"`
			OccurredAt string `json:"occurred_at"`
		}{
			Type:       "payment.refunded",
			PaymentID:  paymentID,
			Amount:     amount,
			Status:     "refunded",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		}
		b, _ := json.Marshal(ev)
		_, _, _ = bl.Producer.SendMessage(&sarama.ProducerMessage{
			Topic: "payments_refunded",
			Value: sarama.ByteEncoder(b),
		})
	}

	return out, nil
}
