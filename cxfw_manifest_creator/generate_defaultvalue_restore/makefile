# Go compiler
GO := go

# Binary name
BINARY := generate_defaultvalues_comparison

# Source file
SRC := main.go

# Build flags
BUILD_FLAGS := -ldflags "-s -w"

# Default target (static build)
all: static

# Compile as a static binary
static:
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o $(BINARY) $(SRC)
	strip $(BINARY)

# Clean up compiled files
clean:
	rm -f $(BINARY)

