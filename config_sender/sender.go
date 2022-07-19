package main

import (
	"agent"
	jsoniter "github.com/json-iterator/go"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"time"
)

var config agent.Config

func failOnError(err error, msg string) {
	if err != nil {
		log.Printf("%s: %s", msg, err)
	}
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"agent_event", // name
		false,         // durable
		true,          // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	failOnError(err, "Failed to declare a queue")

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
	err = ch.Publish(
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

	sleep, _ := time.ParseDuration(config.ReloadTime)
	time.Sleep(sleep)
}
