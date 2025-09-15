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

type bizlogic struct {
	DB       *sql.DB
	Producer sarama.SyncProducer
}

func NewBizLogic(db *sql.DB) *bizlogic {
	return &bizlogic{DB: db}
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

	// Auto-generate order_id if missing: ORD-<n>
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
