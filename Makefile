.PHONY: setup
setup: clean ## Install all the build dependencies
	go get github.com/kardianos/service
	go get github.com/google/gousb
	go get github.com/miekg/pkcs11
	go get github.com/rabbitmq/amqp091-go
	go get golang.org/x/sys

.PHONY: build
build: clean setup ## Build a version
	go build -o ./bin/agent ./agent/service_agent.go
	go build -o ./bin/observer ./observer/service_observer.go

.PHONY: clean
clean: ## Clean build and temporary files
	go clean
ifeq ($(OS),Windows_NT)
	del ./bin
else
	rm -rf ./bin
endif

.PHONY: install
install: install_service_agent install_service_observer ## Install both services

.PHONY: start
start: start_service_agent start_service_observer ## Start both services

.PHONY: stop
stop: stop_service_observer stop_service_agent ## Stop both services

.PHONY: remove
remove: remove_service_observer remove_service_agent ## Remove both services

.PHONY: status
status: ## Status of services
	@./bin/agent status
	@./bin/observer status

.PHONY: restart
restart: clean build ## Restart both services
	@./bin/agent remove
	@./bin/observer remove
	@./bin/agent install
	@./bin/observer install

.PHONY: install_service_observer
install_service_observer: build ## Install observer service
	./bin/observer install

.PHONY: install_service_agent
install_service_agent: build ## Install agent service
	./bin/agent install

.PHONY: start_service_observer
start_service_observer: build ## Start observer service
	./bin/observer start

.PHONY: start_service_agent
start_service_agent: build ## Start agent service
	./bin/agent start

.PHONY: stop_service_observer
stop_service_observer: build ## Stop observer service
	./bin/observer stop

.PHONY: stop_service_agent
stop_service_agent: build ## Stop agent service
	./bin/agent stop

.PHONY: remove_service_observer
remove_service_observer: build ## Remove observer service
	./bin/observer remove

.PHONY: remove_service_agent
remove_service_agent: build ## Remove agent service
	./bin/agent remove

.PHONY: help
help: ## Help
	@echo "//////////////////////////////////////////////////////"
	@echo "// Installing services provides automatic starting. //"
	@echo "// You don't need to start them after installation. //"
	@echo "//////////////////////////////////////////////////////"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help