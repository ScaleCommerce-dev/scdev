# Test commands for testing

# Default recipe
default: unit

# Run all tests
all: unit lint

# Run unit tests
unit:
    @echo "Running unit tests..."

# Run linter
lint:
    @echo "Running linter..."

# Run specific test
run *ARGS:
    @echo "Running tests with args: {{ARGS}}"
