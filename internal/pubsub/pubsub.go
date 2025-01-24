package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DurableQueue = iota
	TransientQueue
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jsonBytes, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("could not marshal date: %v", err)
	}

	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonBytes,
	}
	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, msg)
	if err != nil {
		return fmt.Errorf("could not publish: %v", err)
	}

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {

	channel, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not create channel: %v", err)
	}

	var durable, autoDelete, exclusive bool
	switch simpleQueueType {
	case DurableQueue:
		durable = true
		autoDelete = false
		exclusive = false
	case TransientQueue:
		durable = false
		autoDelete = true
		exclusive = true
	}

	queue, err := channel.QueueDeclare(queueName, durable, autoDelete, exclusive, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not create queue: %v", err)
	}

	err = channel.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not bind queue: %v", err)
	}

	return channel, queue, nil
}
