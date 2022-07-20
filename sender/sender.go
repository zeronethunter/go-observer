package main

import (
	"agent"
	"context"
	jsoniter "github.com/json-iterator/go"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"os"
	"time"
)

var config agent.Config
var conn *amqp.Connection
var ch *amqp.Channel
var q amqp.Queue
var chHandler <-chan amqp.Delivery

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func sendConfig(ctx context.Context) {
	q, err := ch.QueueDeclare(
		"agent_event", // name
		false,         // durable
		true,          // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	failOnError(err, "Failed to declare a queue")

	chHandler = channelHandler()

	tokenDriver := map[string]map[string]string{
		"linux": {
			"0A89": "/home/zenehu/Documents/Work/sdk/pkcs11/lib/linux_glibc-x86_64/librtpkcs11ecp.so",
		},
		"windows": {
			"0A89": "C:\\Windows\\System32\\rtPKCS11.dll",
		},
	}
	// Configure agent
	config = agent.Config{
		TokenDriver:     tokenDriver,
		Path:            "config.json",
		ReloadTime:      "10s",
		PossibleVendors: []string{"0A89"},
	}

	body, err := jsoniter.MarshalIndent(config, "", "    ")

	if err != nil {
		log.Panicf(err.Error())
	}

	err = ch.PublishWithContext(
		ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
			Type:        "config",
		})
	failOnError(err, "Failed to publish a message")
	log.Printf("%s: \n%s", "Sent", body)
}

func sendPing(ctx context.Context) {
	q, err := ch.QueueDeclare(
		"observer_event", // name
		false,            // durable
		true,             // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	failOnError(err, "Failed to declare a queue")

	chHandler = channelHandler()

	if err := ch.PublishWithContext(
		ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte("ping"),
		}); err != nil {
		failOnError(err, "Failed to send ping message")
	}
	log.Printf("%s: \n%s", "Sent", "ping")
}

func channelHandler() <-chan amqp.Delivery {
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	agent.ErrorHandler(err, "Failed to register a consumer")

	return msgs
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err = conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := os.Args[1]

	switch command {
	case "send_config":
		sendConfig(ctx)
	case "ping":
		sendPing(ctx)
	}

	for {
		select {
		case d := <-chHandler:
			if string(d.Body) == "ping" {
				log.Printf("%s: %s", "Error", "Start observer first")
			} else {
				log.Printf("%s: %s", "Response", d.Body)
			}
			os.Exit(0)
		}
	}
}
