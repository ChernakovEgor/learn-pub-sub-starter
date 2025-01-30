package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

	publishChannel, err := conn.Channel()
	if err != nil {
		fmt.Printf("getting channel: %v", err)
		os.Exit(1)
	}

	//subscribe to pause queue
	pauseQueue := routing.PauseKey + "." + username
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, pauseQueue, routing.PauseKey, pubsub.TransientQueue, handlerPause(gameState))
	if err != nil {
		fmt.Printf("subscribing to json: %v", err)
		os.Exit(1)
	}

	// subscribe to moves queue
	movesQueue := "army_moves." + username
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, movesQueue, "army_moves.*", pubsub.TransientQueue, handlerMove(gameState, publishChannel))
	if err != nil {
		fmt.Printf("subscribing to json: %v", err)
		os.Exit(1)
	}

	// subscribe to war queue
	warQueue := "war"
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, warQueue, "war.*", pubsub.DurableQueue, handlerWar(gameState, publishChannel))
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
			err := move(gameState, input, publishChannel, movesQueue)
			if err != nil {
				fmt.Printf("Could not move units: %v\n", err)
			}
			// redo - move to 'move' handler
		case "status":
			status(gameState)
		case "help":
			help()
		case "spam":
			if len(input) < 2 {
				fmt.Println("not enough arguments")
			}
			timesString := input[1]
			spamAmount, err := strconv.Atoi(timesString)
			if err != nil {
				fmt.Println("not a number")
			}

			for range spamAmount {
				msg := gamelogic.GetMaliciousLog()
				err = pubsub.PublishGob(publishChannel, "peril_topic", "game_logs"+"."+gameState.GetUsername(), routing.GameLog{
					CurrentTime: time.Now(),
					Username:    gameState.GetUsername(),
					Message:     msg,
				})
				if err != nil {
					fmt.Printf("spamming message: %v\n", err)
				}
			}
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
func spam(args []string) {
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

func handlerMove(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(channel,
				"peril_topic",
				"war."+gs.GetUsername(),
				gamelogic.RecognitionOfWar{Attacker: move.Player, Defender: gs.GetPlayerSnap()},
			)
			if err != nil {
				log.Printf("failed to publish to war channel: %v", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gamestate *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome, winner, loser := gamestate.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			warLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    gamestate.GetUsername(),
			}
			err := pubsub.PublishGob(channel, "peril_topic", routing.GameLogSlug+"."+rw.Attacker.Username, warLog)
			if err != nil {
				log.Printf("error while publishing gob: %v", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			warLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("%s won a war against %s", winner, loser),
				Username:    gamestate.GetUsername(),
			}
			err := pubsub.PublishGob(channel, "peril_topic", routing.GameLogSlug+"."+rw.Attacker.Username, warLog)
			if err != nil {
				log.Printf("error while publishing gob: %v", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			warLog := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser),
				Username:    gamestate.GetUsername(),
			}
			err := pubsub.PublishGob(channel, "peril_topic", routing.GameLogSlug+"."+rw.Attacker.Username, warLog)
			if err != nil {
				log.Printf("error while publishing gob: %v", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			fmt.Println("war outcome error")
			return pubsub.NackDiscard
		}
	}
}
