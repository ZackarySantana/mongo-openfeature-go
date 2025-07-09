IMAGE_NAME := lidtop/mongo-openfeature-go-editor
TAG := latest
DEV_TAG := dev
DOCKERFILE := Dockerfile.editor

PLATFORMS := linux/amd64,linux/arm64

.PHONY: help
help:
	@echo "Usage: make VERSION=<version> <target>"
	@echo "Example: make VERSION=v1.0.0 docker-publish-editor-prod"
	@echo ""
	@echo "Available commands:"
	@echo "  make docker-build-editor-dev    - Builds the development Docker image for your local architecture."
	@echo "  make docker-publish-editor-prod - Builds and publishes a multi-arch (AMD64, ARM64) production image."
	@echo "  make help                       - Shows this help message."

.PHONY: check-version
check-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is not set. Please use 'VERSION=<version> make <target>'"; \
		exit 1; \
	fi

.PHONY: docker-build-editor-dev
docker-build-editor-dev: check-version
	@echo "--> Building development image for local architecture with tags: $(DEV_TAG) and $(VERSION)-$(DEV_TAG)"
	docker build --target dev -f $(DOCKERFILE) \
		-t $(IMAGE_NAME):$(DEV_TAG) \
		-t $(IMAGE_NAME):$(VERSION)-$(DEV_TAG) .

.PHONY: docker-publish-editor-prod
docker-publish-editor-prod: check-version
	@echo "--> Building and publishing multi-platform ($(PLATFORMS)) image with tags: $(TAG) and $(VERSION)"
	docker buildx build --platform $(PLATFORMS) --target prod -f $(DOCKERFILE) \
		-t $(IMAGE_NAME):$(TAG) \
		-t $(IMAGE_NAME):$(VERSION) \
		--push .

.PHONY: all
all: help

.PHONY: clean
clean:
	@echo "--> Cleaning up local Docker images"
	@docker rmi $(IMAGE_NAME):$(TAG) || true
	@docker rmi $(IMAGE_NAME):$(DEV_TAG) || true