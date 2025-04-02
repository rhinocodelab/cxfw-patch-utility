APP_NAME = cxfw_patch_rollback
GO_FILES = $(shell find . -type f -name '*.go')
OUTPUT_DIR = .
OUTPUT_FILE = $(OUTPUT_DIR)/$(APP_NAME)

.PHONY: all clean build run

all: build

build: $(OUTPUT_FILE)

$(OUTPUT_FILE): $(GO_FILES)
	@mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(OUTPUT_FILE) .
	strip $(OUTPUT_FILE)
	@echo "Build complete: $(OUTPUT_FILE) (Stripped & Optimized)"

clean:
	@rm -rf $(OUTPUT_FILE)
	@echo "Cleaned up build files."

run: build
	$(OUTPUT_FILE)
