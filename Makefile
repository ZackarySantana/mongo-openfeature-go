IMAGE_NAME := lidtop/mongo-openfeature-go-editor
TAG := latest
DEV_TAG := dev
DOCKERFILE := Dockerfile.editor

.PHONY: help
help:
	@echo "Usage: make VERSION=<version> <target>"
	@echo "Example: make VERSION=v1.0.0  docker-publish-editor-prod"
	@echo ""
	@echo "Available commands:"
	@echo "  make docker-build-editor-dev    - Builds the development Docker image."
	@echo "  make docker-build-editor-prod   - Builds the production Docker image."
	@echo "  make docker-publish-editor-prod - Publishes the production image (tags: latest, <version>)."
	@echo "  make help                       - Shows this help message."

# This is a guard target. It ensures VERSION is set for any target that depends on it.
.PHONY: check-version
check-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is not set. Please use 'VERSION=<version> make <target>'"; \
		exit 1; \
	fi

.PHONY: docker-build-editor-dev
docker-build-editor-dev: check-version
	@echo "--> Building development image with tags: $(DEV_TAG) and $(VERSION)-$(DEV_TAG)"
	docker build --target dev -f $(DOCKERFILE) \
		-t $(IMAGE_NAME):$(DEV_TAG) \
		-t $(IMAGE_NAME):$(VERSION)-$(DEV_TAG) .

.PHONY: docker-build-editor-prod
docker-build-editor-prod: check-version
	@echo "--> Building production image with tags: $(TAG) and $(VERSION)"
	docker build --target prod -f $(DOCKERFILE) \
		-t $(IMAGE_NAME):$(TAG) \
		-t $(IMAGE_NAME):$(VERSION) .

.PHONY: docker-publish-editor-prod
docker-publish-editor-prod: check-version docker-build-editor-prod
	@echo "--> Publishing production image tags: $(TAG) and $(VERSION)"
	docker push $(IMAGE_NAME):$(TAG)
	docker push $(IMAGE_NAME):$(VERSION)

.PHONY: all
all: docker-build-editor-prod

.PHONY: clean
clean:
	@echo "--> Cleaning up local Docker images"
	@docker rmi $(IMAGE_NAME):$(TAG) || true
	@docker rmi $(IMAGE_NAME):$(DEV_TAG) || true