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
	gameState := gamelogic.NewGameState(username)

	queueName := routing.PauseKey + "." + username
	_, _, err = pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TransientQueue)
	if err != nil {
		fmt.Printf("could not bind queue: %v", err)
		os.Exit(1)
	}

	//subscribe to pause queue
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TransientQueue, handlerPause(gameState))
	if err != nil {
		fmt.Printf("subscribing to json: %v", err)
		os.Exit(1)
	}

	// create army_moves queue
	movesQueue := "army_moves." + username
	movesChannel, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, movesQueue, "army_moves.*", pubsub.TransientQueue)
	if err != nil {
		fmt.Printf("could not bind moves queue: %v", err)
		os.Exit(1)
	}
	// subscribe to moves queue
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, movesQueue, "army_moves.*", pubsub.TransientQueue, handlerMove(gameState))
	if err != nil {
		fmt.Printf("subscribing to json: %v", err)
		os.Exit(1)
	}
	// subscribe to moves queue
	warQueue := "war"
	warKey := routing.WarRecognitionsPrefix + "." + username
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, warQueue, "war.*", pubsub.DurableQueue, handlerWar(gameState))
	if err != nil {
		fmt.Printf("subscribing to json: %v", err)
		os.Exit(1)
	}

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
			err := move(gameState, input, movesChannel, movesQueue)
			if err != nil {
				fmt.Printf("Could not move units: %v\n", err)
			}
			// redo - move to 'move' handler
			pubsub.PublishJSON(movesChannel, "peril_topic", warKey, gamelogic.RecognitionOfWar{})
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

func move(gamestate *gamelogic.GameState, args []string, channel *amqp.Channel, key string) error {
	move, err := gamestate.CommandMove(args)
	if err != nil {
		return err
	}
	err = pubsub.PublishJSON(channel, routing.ExchangePerilTopic, key, move)
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(state routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(state)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			return pubsub.NackRequeue
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}

		return pubsub.NackDiscard
	}
}

func handlerWar(gamestate *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome, _, _ := gamestate.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			fmt.Println("war outcome error")
			return pubsub.NackDiscard
		}
		return pubsub.Ack
	}
}
