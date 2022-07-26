package main

import (
	"agent"
	"github.com/kardianos/service"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var stdlog, errlog *log.Logger

var config = map[string]string{
	"port":             ":9977", // port to ping
	"period":           "5s",    // duration between pings
	"service_to_serve": "agent", // name of service to observe
}

type program struct{}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) run() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	// RabbitMQ connection
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	agent.ErrorHandler(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	// RabbitMQ channel
	channel, err := conn.Channel()
	agent.ErrorHandler(err, "Failed to open a channel")
	defer channel.Close()

	q, err := channel.QueueDeclare(
		"observer_event", // name
		false,            // durable
		true,             // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)

	pingChannel := pingHandler(channel, q)

	for {
		select {
		case d := <-pingChannel:
			if string(d.Body) == "ping" {
				state := ping(interrupt)
				stdlog.Printf("%s: %s\n", "Signal", d.Body)
				pingResponse(channel, q, state)
			}

		case killSignal := <-interrupt:
			stdlog.Println("Got signal:", killSignal)
			if killSignal == os.Interrupt {
				stdlog.Println("Daemon was interrupted by system signal")
				return
			}
			stdlog.Println("Daemon was killed")
			return
		}
	}
}

func pingHandler(channel *amqp.Channel, queue amqp.Queue) <-chan amqp.Delivery {
	msgs, err := channel.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	agent.ErrorHandler(err, "Failed to register a consumer")

	return msgs
}

func pingResponse(channel *amqp.Channel, queue amqp.Queue, msg string) {
	if err := channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(msg),
		}); err != nil {
		agent.ErrorHandler(err, "Failed to send ping message")
	}
}

func ping(interrupt chan os.Signal) string {
	_, err := net.Dial("tcp", config["port"])
	if err != nil {
		args := []string{"service"}

		var sc = &service.Config{
			Name:      config["service_to_serve"],
			Arguments: args,
		}

		prg := &program{}
		s, err := service.New(prg, sc)
		if err != nil {
			errlog.Println("Failed to create instance "+config["service_to_serve"]+": ", err)
			interrupt <- os.Kill
		}
		err = s.Start()
		if err != nil {
			return "Failed to start " + config["service_to_serve"] + ": " + err.Error()
		} else {
			return "Service " + config["service_to_serve"] + " successfully started"
		}
	}
	return "Service " + config["service_to_serve"] + " is running..."
}

func (p *program) Stop(s service.Service) error {
	return nil
}

func init() {
	stdlog = log.New(os.Stdout, "", 0)
	errlog = log.New(os.Stderr, "", 0)
}

func main() {
	args := []string{"service"}

	options := map[string]interface{}{
		"Restart": "on-failure",
	}

	var sc = &service.Config{
		Name:        "observer",
		DisplayName: "observer",
		Description: "Observer of agent service",
		Option:      options,
		Arguments:   args,
	}

	// Service configuration
	prg := &program{}
	s, err := service.New(prg, sc)
	if err != nil {
		errlog.Println("Failed to create service ", sc.Name, ": ", err)
		os.Exit(1)
	}

	// Command argument of service
	cmdArg := os.Args[1]

	switch cmdArg {
	case "install":
		err = s.Install()
		if err != nil {
			errlog.Println("Failed to install "+sc.Name+": ", err)
			os.Exit(1)
		}
		err = s.Start()
		if err != nil {
			errlog.Println("Failed to start "+sc.Name+": ", err)
			os.Exit(1)
		}

		os.Exit(0)
	case "start":
		err = s.Start()
		if err != nil {
			errlog.Println("Failed to start "+sc.Name+": ", err)
			os.Exit(1)
		}

		os.Exit(0)
	case "stop":
		err = s.Stop()
		if err != nil {
			errlog.Println("Failed to stop "+sc.Name+": ", err)
			os.Exit(1)
		}

		os.Exit(0)
	case "remove":
		err = s.Stop()
		if err != nil {
			errlog.Print("Failed to stop "+sc.Name+": ", err)
			os.Exit(1)
		}

		err = s.Uninstall()
		if err != nil {
			errlog.Print("Failed to remove "+sc.Name+": ", err)
			os.Exit(1)
		}

		os.Exit(0)
	case "status":
		status, err := s.Status()
		if err != nil {
			errlog.Print("Failed to get status of "+sc.Name+": ", err)
			os.Exit(1)
		}
		switch status {
		case service.StatusUnknown:
			errlog.Print("Failed to get status of "+sc.Name+": ", "Unable to be determined due to an error or it was not installed")
			os.Exit(1)
		case service.StatusRunning:
			stdlog.Print("Status: [", sc.Name, "] is currently RUNNING on system")
		case service.StatusStopped:
			stdlog.Print("Status: [", sc.Name, "] is currently STOPPED on system")
		}
		os.Exit(0)
	case "service":
		err = s.Run()
		if err != nil {
			errlog.Print("Failed to run "+sc.Name+": ", err)
			os.Exit(1)
		}
	default:
		errlog.Println("Unrecognized command: " + cmdArg)
		errlog.Println("Usage: <service> install | remove | start | stop | status")
		os.Exit(1)
	}
}
