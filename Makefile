.PHONY: dev build clean install test run-backend run-frontend

# Development - runs both backend and frontend in development mode
dev:
	@echo "Starting development servers..."
	@cd backend && air &
	@cd frontend && npm run dev

# Install dependencies
install:
	@echo "Installing dependencies..."
	@cd backend && go mod tidy
	@cd frontend && npm install

# Build the project
build:
	@echo "Building project..."
	@cd frontend && npm run build
	@cd backend && go build -o goblons main.go

# Run only the backend server
run-backend:
	@echo "Starting backend server..."
	@cd backend && go run main.go

# Run only the frontend development server
run-frontend:
	@echo "Starting frontend development server..."
	@cd frontend && npm run dev

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@cd backend && rm -f goblons
	@cd frontend && rm -rf node_modules dist

# Run tests
test:
	@echo "Running tests..."
	@cd backend && go test ./...

# Production build and run
prod:
	@echo "Building for production..."
	@cd frontend && npm run build
	@cd backend && go build -o goblons main.go
	@echo "Starting production server..."
	@cd backend && ./goblons

# Development with hot reload (requires air for Go)
dev-hot:
	@echo "Starting development with hot reload..."
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/cosmtrek/air@latest; }
	@cd backend && air &
	@cd frontend && npm run dev

# Help
help:
	@echo "Available commands:"
	@echo "  make install    - Install all dependencies"
	@echo "  make dev        - Start development servers"
	@echo "  make dev-hot    - Start with hot reload (installs air if needed)"
	@echo "  make build      - Build the project"
	@echo "  make run-backend - Run only backend"
	@echo "  make run-frontend - Run only frontend"
	@echo "  make prod       - Build and run in production mode"
	@echo "  make test       - Run tests"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make help       - Show this help"
