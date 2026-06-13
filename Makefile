# Shark Dashboard - Makefile
# Build for ARM64 (Termux/Android) with memory-safe settings

BINARY_NAME=shark-dashboard
BUILD_DIR=build

# ARM64 cross-compilation
GOOS=linux
GOARCH=arm64

# Memory-safe build flags for constrained devices
LDFLAGS=-s -w

.PHONY: all build-arm64 build-local clean run

all: build-arm64

build-arm64:
	@echo "🦈 Building $(BINARY_NAME) for ARM64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -p 2 -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 .
	@echo "✅ Built: $(BUILD_DIR)/$(BINARY_NAME)-arm64"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-arm64

build-local:
	@echo "🦈 Building $(BINARY_NAME) for local testing..."
	@mkdir -p $(BUILD_DIR)
	go build -p 2 -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "✅ Built: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	rm -rf $(BUILD_DIR)

run: build-local
	./$(BUILD_DIR)/$(BINARY_NAME) -port 8081
