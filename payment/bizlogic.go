package payment

import (
	"database/sql"
	"payment/model"

	"github.com/IBM/sarama"
)

type IbizLogic interface {
	RefundPaymentLogic(paymentID int64) (model.RefundResponse, error)
}

type bizlogic struct {
	DB       *sql.DB
	Producer sarama.SyncProducer
}

func NewBizLogic(db *sql.DB, producer sarama.SyncProducer) *bizlogic {
	return &bizlogic{DB: db, Producer: producer}
}
