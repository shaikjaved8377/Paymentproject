package payment

import (
	"database/sql"
	"net/http"

	"github.com/IBM/sarama"
)

func RefundRoutes(db *sql.DB, producer sarama.SyncProducer) {
	h := NewHandler(db, producer)
	http.HandleFunc("/create", h.RefundHandler())

}
