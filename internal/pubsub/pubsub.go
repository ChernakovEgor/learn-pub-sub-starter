package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DurableQueue = iota
	TransientQueue
)

type Acktype int

func (a Acktype) String() string {
	switch a {
	case Ack:
		return "Ack"
	case NackRequeue:
		return "NackRequeue"
	case NackDiscard:
		return "NackDiscard"
	}
	return "InvalidAcktype"
}

const (
	Ack Acktype = iota
	NackRequeue
	NackDiscard
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
) error {

	channel, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return fmt.Errorf("binding quque: %v", err)
	}

	// channel.Qos(10, 0, false)
	deliveries, err := channel.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("creating consume channlel: %v", err)
	}

	go func() {
		for d := range deliveries {
			var message T
			err := json.Unmarshal(d.Body, &message)
			if err != nil {
				fmt.Printf("could not unmarshal message: %v\n", err)
			}
			res := handler(message)
			log.Printf("acknowledge status: %v\n", res)

			switch res {
			case Ack:
				err = d.Ack(false)
				if err != nil {
					fmt.Printf("could not ack message: %v\n", err)
				}
			case NackRequeue:
				err = d.Nack(false, true)
				if err != nil {
					fmt.Printf("could not requeue message: %v\n", err)
				}
			case NackDiscard:
				err = d.Nack(false, false)
				if err != nil {
					fmt.Printf("could not discard message: %v\n", err)
				}
			}
		}
	}()

	return nil
}

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

	table := make(amqp.Table)
	table["x-dead-letter-exchange"] = "peril_dlx"
	queue, err := channel.QueueDeclare(queueName, durable, autoDelete, exclusive, false, table)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not create queue: %v", err)
	}

	err = channel.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not bind queue: %v", err)
	}

	return channel, queue, nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(val)
	if err != nil {
		return fmt.Errorf("encoding to gob: %v", err)
	}

	msg := amqp.Publishing{
		ContentType: "application/gob",
		Body:        buffer.Bytes(),
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, msg)
	if err != nil {
		return fmt.Errorf("publishing gob: %v", err)
	}
	return nil
}

func SubscribeGob[T any](conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
) error {

	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, func(data []byte) (T, error) {
		buffer := bytes.NewBuffer(data)
		decoder := gob.NewDecoder(buffer)
		var msg T
		err := decoder.Decode(&msg)
		return msg, err
	})
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
	unmarshaller func([]byte) (T, error),
) error {

	channel, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return fmt.Errorf("binding to queue: %v", err)
	}

	// channel.Qos(10, 0, false)
	deliveries, err := channel.Consume(queueName, "", false, false, false, false, nil)
	go func() {
		for d := range deliveries {
			msg, err := unmarshaller(d.Body)
			if err != nil {
				log.Printf("unmarshaling delivery: %v", err)
			}

			ackStatus := handler(msg)
			switch ackStatus {
			case Ack:
				err := d.Ack(false)
				if err != nil {
					log.Printf("could not ack delivery: %v", err)
				}
			case NackDiscard:
				err := d.Nack(false, false)
				if err != nil {
					log.Printf("could not ack delivery: %v", err)
				}
			case NackRequeue:
				err := d.Nack(false, true)
				if err != nil {
					log.Printf("could not ack delivery: %v", err)
				}
			default:
				err := d.Nack(false, true)
				if err != nil {
					log.Printf("could not ack delivery: %v", err)
				}
			}
		}
	}()

	return nil
}
