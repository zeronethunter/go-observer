package main

import (
	"github.com/kardianos/service"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	go ping(interrupt)

	for {
		select {
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

func ping(interrupt chan os.Signal) {
	for {
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
				errlog.Print("Failed to create instance " + config["service_to_serve"] + ": ")
				errlog.Println(err)
				interrupt <- os.Kill
			}
			err = s.Start()
			if err != nil {
				errlog.Print("Failed to start " + config["service_to_serve"] + ": ")
				errlog.Println(err)
			} else {
				stdlog.Println("Service " + config["service_to_serve"] + " successfully started")
			}
		}

		stdlog.Println("Service " + config["service_to_serve"] + " is running...")
		wait, err := time.ParseDuration(config["period"])
		if err != nil {
			errlog.Println(err)
			interrupt <- os.Kill
			return
		}
		time.Sleep(wait)
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
		Name:        "observer",
		DisplayName: "observer",
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
