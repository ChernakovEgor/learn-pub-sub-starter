package main

import (
	"fmt"
	"os"
	// "os/signal"

	"github.com/ChernakovEgor/learn-pub-sub-starter/internal/gamelogic"
	"github.com/ChernakovEgor/learn-pub-sub-starter/internal/pubsub"
	"github.com/ChernakovEgor/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const CONN = "amqp://guest:guest@localhost:5672/"

func main() {
	conn, err := amqp.Dial(CONN)
	if err != nil {
		fmt.Printf("could not connect to server: %v", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("Starting Peril server...")

	channel, err := conn.Channel()
	if err != nil {
		fmt.Printf("could not create channel: %v\n", err)
		os.Exit(1)
	}

	_, _, err = pubsub.DeclareAndBind(conn, "peril_topic", "game_logs", "game_logs.*", pubsub.DurableQueue)
	if err != nil {
		fmt.Printf("could not create durable queue: %v", err)
		os.Exit(1)
	}

	gamelogic.PrintServerHelp()
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "pause":
			pause(channel)
		case "resume":
			resume(channel)
		case "quit":
			quit()
		default:
			gamelogic.PrintServerHelp()
		}
	}
}

func pause(channel *amqp.Channel) {
	fmt.Println("Game paused")
	playingState := routing.PlayingState{IsPaused: true}
	err := pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, playingState)
	if err != nil {
		fmt.Printf("could not publish json: %v", err)
		os.Exit(1)
	}
}
func resume(channel *amqp.Channel) {
	fmt.Println("Game resumed")
	playingState := routing.PlayingState{IsPaused: false}
	err := pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, playingState)
	if err != nil {
		fmt.Printf("could not publish json: %v", err)
		os.Exit(1)
	}
}
func quit() {
	fmt.Println("Shutting down...")
	os.Exit(0)
}
