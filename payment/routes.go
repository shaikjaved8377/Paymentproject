package payment

import (
	"database/sql"
	"net/http"

	"github.com/IBM/sarama"
)

func RegisterRoutes(mux *http.ServeMux, db *sql.DB, producer sarama.SyncProducer, topic string) {
	svc := NewService(db, producer, topic)
	h := NewHandler(svc)

	
	mux.HandleFunc("/v1/payments/authorize", h.Authorize())
}
