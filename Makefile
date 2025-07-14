# Image names
IMAGE_EDITOR := lidtop/mongo-openfeature-go-editor
IMAGE_MCP    := lidtop/mongo-openfeature-go-mcp-server

# Tags
TAG         := latest

# Dockerfiles
DOCKERFILE_EDITOR := Dockerfile.editor
DOCKERFILE_MCP    := Dockerfile.mcp

# Multi-arch platforms
PLATFORMS   := linux/amd64,linux/arm64

.PHONY: help
help:
	@echo "Usage: make VERSION=<version> <target>"
	@echo ""
	@echo "Available commands:"
	@echo "  docker-publish-editor        - Build & publish editor image (multi-arch)."
	@echo "  docker-publish-mcp           - Build & publish mcp-server image (multi-arch)."
	@echo "  build-mcp-windows-arm64      - Builds the mcp.exe binary for Windows/ARM64."
	@echo "  clean                        - Remove local images."
	@echo "  help                         - This help message."

.PHONY: check-version
check-version:
	@if [ -z "$(VERSION)" ]; then \
	  echo "Error: VERSION must be set, e.g. VERSION=v1.0.0"; \
	  exit 1; \
	fi

.PHONY: docker-publish-editor
docker-publish-editor: check-version
	@echo "--> Building & publishing editor image ($(PLATFORMS)) with tags: $(TAG), $(VERSION)"
	docker buildx build \
	  --platform $(PLATFORMS) \
	  -f $(DOCKERFILE_EDITOR) \
	  -t $(IMAGE_EDITOR):$(TAG) \
	  -t $(IMAGE_EDITOR):$(VERSION) \
	  --push .

.PHONY: docker-publish-mcp
docker-publish-mcp: check-version
	@echo "--> Building & publishing mcp-server image ($(PLATFORMS)) with tags: $(TAG), $(VERSION)"
	docker buildx build \
	  --platform $(PLATFORMS) \
	  -f $(DOCKERFILE_MCP) \
	  -t $(IMAGE_MCP):$(TAG) \
	  -t $(IMAGE_MCP):$(VERSION) \
	  --push .

# This target is useful for wsl2 users on arm64 machines who want to run the mcp-server
# binary on Windows with an mcp client.
.PHONY: build-mcp-windows-amd64
build-mcp-windows-arm64:
	@echo "--> Building mcp.exe for windows/arm64"
	GOOS=windows GOARCH=arm64 go build -o mcp.exe cmd/mcp/main.go

.PHONY: clean
clean:
	@echo "--> Cleaning up local Docker images"
	-@docker rmi $(IMAGE_EDITOR):$(TAG) $(IMAGE_EDITOR):$(VERSION) || true
	-@docker rmi $(IMAGE_MCP):$(TAG)    $(IMAGE_MCP):$(VERSION)    || true