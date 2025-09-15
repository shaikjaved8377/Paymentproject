package payment

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"payment/model"

	"github.com/IBM/sarama"
)

type Handler struct {
	Svc *Service
	biz IbizLogic
}

func NewHandler(svc *Service) *Handler { return &Handler{Svc: svc} }

func (h *Handler) Authorize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req model.AuthorizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		resp, err := h.Svc.Authorize(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

}

func NewRefundHandler(db *sql.DB, producer sarama.SyncProducer) *Handler {
	return &Handler{biz: NewBizLogic(db, producer)}
}

func (h *Handler) RefundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req model.RefundReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PaymentID == "" {
			http.Error(w, "invalid json or payment_id", http.StatusBadRequest)
			return
		}

		resp, err := h.biz.RefundPaymentLogic(req.PaymentID)
		if err != nil {
			if err.Error() == "payment not eligible for refund" {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
