package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/IBM/sarama"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := "username:password@tcp(localhost:3306)/sys?parseTime=true"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	producer, err := initkafkaproducer()
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	defer producer.Close()

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
