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
	fmt.Println("Starting Peril client...")
	conn, err := amqp.Dial(CONN)
	if err != nil {
		fmt.Printf("could not dial server: %v", err)
		os.Exit(1)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("could not get username: %v", err)
		os.Exit(1)
	}

	queueName := routing.PauseKey + "." + username
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TransientQueue)
	if err != nil {
		fmt.Printf("could not bind queue: %v", err)
		os.Exit(1)
	}

	gameState := gamelogic.NewGameState(username)
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "spawn":
			err := spawn(gameState, input)
			if err != nil {
				fmt.Printf("Could not spawn units: %v\n", err)
			}
		case "move":
			err := move(gameState, input)
			if err != nil {
				fmt.Printf("Could not move units: %v\n", err)
			}
		case "status":
			status(gameState)
		case "help":
			help()
		case "spam":
			spam()
		case "quit":
			quit()
		default:
			gamelogic.PrintClientHelp()
		}
	}
}

func spawn(gamestate *gamelogic.GameState, args []string) error {
	err := gamestate.CommandSpawn(args)
	if err != nil {
		return err
	}
	return nil
}

func move(gamestate *gamelogic.GameState, args []string) error {
	_, err := gamestate.CommandMove(args)
	if err != nil {
		return err
	}
	fmt.Println("Moved armies")
	return nil
}

func status(gamestate *gamelogic.GameState) {
	gamestate.CommandStatus()
}
func help() {
	gamelogic.PrintClientHelp()
}
func spam() {
	fmt.Println("Spamming not allowed!")
}
func quit() {
	gamelogic.PrintQuit()
	os.Exit(0)
}
