package main

import (
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"os"
	"os/signal"
)

const CONN = "amqp://guest:guest@localhost:5672/"

func main() {
	conn, err := amqp.Dial(CONN)
	if err != nil {
		fmt.Printf("could not connect to server: %v", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Connection successfull")
	fmt.Println("Starting Peril server...")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	fmt.Println("Shutting down...")
}
