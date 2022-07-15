package main

import (
	"github.com/kardianos/service"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var stdlog, errlog *log.Logger

var config = map[string]string{
	"port":             ":9977",
	"period":           "20s",
	"service_to_serve": "handler",
}

type program struct{}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) run() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Set up listener for defined host and port
	listener, err := net.Listen("tcp", config["port"])
	if err != nil {
		errlog.Println("Possibly was a problem with the port binding")
		return
	}

	// set up channel on which to send accepted connections
	ping := make(chan net.Conn, 100)
	go acceptPing(listener, ping)

	// loop work cycle with accept connections or interrupt
	// by system signal
	for {
		select {
		case _ = <-ping:
			continue
		case killSignal := <-interrupt:
			stdlog.Println("Got signal: ", killSignal)
			stdlog.Println("Stoping listening on: ", listener.Addr())
			err := listener.Close()
			if err != nil {
				errlog.Println("Failed to close listener: ", err)
			}
			if killSignal == os.Interrupt {
				stdlog.Println("Daemon was interrupted by system signal")
				return
			}
			stdlog.Println("Daemon was killed")
			return
		}
	}
}

func acceptPing(listener net.Listener, ping chan<- net.Conn) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		ping <- conn
	}
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

	var sc = &service.Config{
		Name:        "handler",
		DisplayName: "handler",
		Description: "Example of a go service",
		Arguments:   args,
	}

	prg := &program{}
	s, err := service.New(prg, sc)
	if err != nil {
		errlog.Println("Failed to create service ", sc.Name, ": ", err)
		os.Exit(1)
	}

	cmdArg := os.Args[1]

	switch cmdArg {
	case "install":
		err = s.Install()
		if err != nil {
			errlog.Println("Failed to install "+sc.Name+": ", err)
		}
		err = s.Start()
		if err != nil {
			errlog.Println("Failed to start "+sc.Name+": ", err)
		}

		os.Exit(0)
	case "start":
		err = s.Start()
		if err != nil {
			errlog.Println("Failed to start "+sc.Name+": ", err)
		}

		os.Exit(0)
	case "stop":
		err = s.Stop()
		if err != nil {
			errlog.Println("Failed to stop "+sc.Name+": ", err)
		}

		os.Exit(0)
	case "remove":
		err = s.Stop()
		if err != nil {
			errlog.Print("Failed to stop "+sc.Name+": ", err)
		}

		err = s.Uninstall()
		if err != nil {
			errlog.Print("Failed to remove "+sc.Name+": ", err)
		}

		os.Exit(0)
	case "service":
		err = s.Run()
		if err != nil {
			errlog.Print("Failed to run "+sc.Name+": ", err)
		}
	default:
		errlog.Println("Unrecognized command: " + cmdArg)
		errlog.Println("Usage: <service> install | remove | start | stop | status")
		os.Exit(0)
	}
}
