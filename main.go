package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/IBM/sarama"
	_ "github.com/go-sql-driver/mysql"

	"payment/dataservice"
	"payment/payment"
)

func main() {
	
	dsn := "root:password@tcp(localhost:3306)/payments?parseTime=true&multiStatements=true"

	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	
	if err := dataservice.InitDB(context.Background(), db); err != nil {
		log.Fatal("init db:", err)
	}

	
	producer, err := initkafkaproducer()
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	defer producer.Close()

	
	const topicAuthorized = "payments_authorized"
	payment.RegisterRoutes(http.DefaultServeMux, db, producer, topicAuthorized)

	fmt.Println("Connected to the database successfully!")
	log.Println("Starting the server on port 8080...")
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initkafkaproducer() (sarama.SyncProducer, error) {
	brokerlist := []string{"localhost:9092"}
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true 
	producer, err := sarama.NewSyncProducer(brokerlist, config)
	if err != nil {
		return nil, err
	}
	fmt.Println("Kafka producer initialized successfully")
	return producer, nil
}
