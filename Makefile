# Makefile

PROJECT_NAME := prometheus-telegram-bot
DOCKER_IMAGE := $(PROJECT_NAME)
GO_BUILD_OUTPUT := $(PROJECT_NAME)

.PHONY: all build docker run clean

all: build docker

build:
	@echo "Building go binary..."
	go build -o ./$(GO_BUILD_OUTPUT) ./cmd/main.go
	@echo "Build finished."

docker:
	@echo "Building docker image..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built: $(DOCKER_IMAGE)"

run:
	@echo "Running docker container..."
	docker run -d \
		-e PROMETHEUS_URL="${PROMETHEUS_URL}" \
		-e BOT_TOKEN="${BOT_TOKEN}" \
        -e PAGE_SIZE="${PAGE_SIZE}" \
		--name $(PROJECT_NAME) \
		$(DOCKER_IMAGE)
    @echo "Container running: $(PROJECT_NAME)"

clean:
	@echo "Cleaning up..."
	rm -f $(GO_BUILD_OUTPUT)
	@echo "Cleaned."
	
stop:
	@echo "Stopping docker container..."
	docker stop $(PROJECT_NAME)
    @echo "Container stopped: $(PROJECT_NAME)"
    
remove:
	@echo "Removing docker container..."
	docker rm $(PROJECT_NAME)
    @echo "Container removed: $(PROJECT_NAME)"

remove_image:
	@echo "Removing docker image..."
	docker rmi $(DOCKER_IMAGE)
    @echo "Image removed: $(DOCKER_IMAGE)"
