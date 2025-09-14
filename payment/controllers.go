package payment

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"payment/model"

	"github.com/IBM/sarama"
)

type Handler struct {
	biz IbizLogic
}

func NewHandler(db *sql.DB, producer sarama.SyncProducer) Handler {
	return Handler{biz: NewBizLogic(db, producer)}
}

// No need to accept db here; it's already in h.biz
func (h Handler) RefundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req model.RefundReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json or payment_id", http.StatusBadRequest)
			return
		}

		_, err := h.biz.RefundPaymentLogic(req.PaymentID) // ✅ ignore first value
		if err != nil {
			switch err.Error() {
			case "payment not eligible for refund":
				http.Error(w, err.Error(), http.StatusConflict)
			default:
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Refund processed successfully"))

	}
}
