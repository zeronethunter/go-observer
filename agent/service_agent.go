package main

import (
	"agent"
	"github.com/kardianos/service"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var stdlog, errlog *log.Logger

var config = map[string]string{
	"port": ":9977", // port for observer service to ping
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

	// Running our agent
	agent.RunAgent()

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

	options := map[string]interface{}{
		"Restart": "on-failure",
	}

	var sc = &service.Config{
		Name:        "agent",
		DisplayName: "agent",
		Description: "Token agent",
		Option:      options,
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
