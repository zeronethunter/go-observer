.PHONY: setup
setup: clean ## Install all the build dependencies
	go get -u github.com/kardianos/service
	go get -u golang.org/x/sys

.PHONY: build
build: clean setup ## Build a version
	go build -o ./bin/handler ./handler/service_handler.go
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
install: install_service_handler install_service_observer ## Install both services

.PHONY: start
start: start_service_handler start_service_observer ## Start both services

.PHONY: stop
stop: stop_service_observer stop_service_handler ## Stop both services

.PHONY: remove
remove: remove_service_observer remove_service_handler ## Remove both services

.PHONY: status
status: build ## Status of services
	./bin/handler status
	./bin/observer status

.PHONY: install_service_observer
install_service_observer: build ## Install observer service
	./bin/observer install

.PHONY: install_service_handler
install_service_handler: build ## Install handler service
	./bin/handler install

.PHONY: start_service_observer
start_service_observer: build ## Start observer service
	./bin/observer start

.PHONY: start_service_handler
start_service_handler: build ## Start handler service
	./bin/handler start

.PHONY: stop_service_observer
stop_service_observer: build ## Stop observer service
	./bin/observer start

.PHONY: stop_service_handler
stop_service_handler: build ## Stop handler service
	./bin/handler start

.PHONY: remove_service_observer
remove_service_observer: build ## Remove observer service
	./bin/observer remove

.PHONY: remove_service_handler
remove_service_handler: build ## Remove handler service
	./bin/handler remove

.PHONY: help
help: ## Help
	@echo "//////////////////////////////////////////////////////"
	@echo "// Installing services provides automatic starting. //"
	@echo "// You don't need to start them after installation. //"
	@echo "//////////////////////////////////////////////////////"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help