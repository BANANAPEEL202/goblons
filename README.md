# Goblons.io - Multiplayer IO Game

A real-time multiplayer IO game similar to doblons.io, built with Go backend and HTML5 Canvas frontend.

## Features

- **Real-time multiplayer**: WebSocket-based communication
- **Player vs Player combat**: Larger players can absorb smaller ones
- **Collectible items**: Food, power-ups, and special items
- **Dynamic world**: Items spawn continuously around the map
- **Responsive controls**: WASD or arrow keys for movement
- **Live score tracking**: Real-time leaderboard
- **Modern development setup**: Hot reloading and modern build tools

## Game Mechanics

## 🎮 **Game Controls**
- **WASD** or **Arrow Keys** - Move your player
- **Mouse** - Aim (for future features)
- **F11** - Toggle fullscreen mode
- **Growth**: Collect food items to grow larger
- **Combat**: Absorb smaller players to gain their size and score
- **Items**: Collect various power-ups:
  - 🟡 Food: Basic growth
  - 💰 Coins: Score points
  - ❤️ Health packs: Restore health
  - 🔵 Size boost: Instant growth
  - ⚡ Speed boost: Temporary speed increase
  - ⭐ Score multiplier: Double your score

## Project Structure

```
goblons/
├── backend/                 # Go server
│   ├── internal/
│   │   ├── game/           # Game logic and mechanics
│   │   │   ├── constants.go
│   │   │   ├── types.go
│   │   │   ├── world.go
│   │   │   └── mechanics.go
│   │   └── server/         # HTTP and WebSocket handling
│   │       └── server.go
│   ├── static/             # Compiled frontend assets
│   ├── main.go            # Server entry point
│   ├── go.mod
│   └── go.sum
├── frontend/               # HTML5 frontend
│   ├── index.html         # Game UI
│   ├── client.js          # Game client logic
│   ├── package.json
│   └── vite.config.js     # Build configuration
└── Makefile              # Build and development commands
```

## Quick Start

### Prerequisites

- Go 1.19+ 
- Node.js 16+
- npm

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd goblons
   ```

2. **Install dependencies**
   ```bash
   make install
   ```

3. **Start development servers**
   ```bash
   make dev
   ```

   This will start:
   - Backend server on `http://localhost:8080`
   - Frontend development server on `http://localhost:3000`

4. **Open your browser**
   - Visit `http://localhost:3000` to play
   - The game runs in **fullscreen mode** for an immersive experience
   - Press **F11** to toggle browser fullscreen
   - Multiple players can join by opening multiple browser tabs/windows

## Development Commands

```bash
# Install all dependencies
make install

# Start development servers (backend + frontend)
make dev

# Start with hot reloading (installs air if needed)
make dev-hot

# Build the project
make build

# Run only backend server
make run-backend

# Run only frontend development server  
make run-frontend

# Build and run in production mode
make prod

# Run tests
make test

# Clean build artifacts
make clean

# Show available commands
make help
```

## Architecture

### Backend (Go)
- **WebSocket server**: Real-time communication
- **Game world simulation**: 60 FPS game loop
- **Player management**: Connection handling and state sync
- **Collision detection**: Player vs player and item collection
- **Item spawning system**: Dynamic world population

### Frontend (JavaScript)
- **HTML5 Canvas rendering**: Smooth 60 FPS graphics
- **WebSocket client**: Real-time server communication
- **Input handling**: Keyboard and mouse controls
- **Camera system**: Follow player with smooth scrolling
- **UI elements**: Score, connection status, controls

### Communication Protocol
- **JSON-based messages** over WebSocket
- **Message types**:
  - `input`: Player controls
  - `snapshot`: Complete game state
  - `playerJoined`/`playerLeft`: Connection events
  - `gameEvent`: Special game events

## Development Features

- **Hot reloading**: Both frontend and backend support live reloading
- **Modern build system**: Vite for frontend, Go modules for backend
- **Development proxy**: Frontend proxies WebSocket to backend
- **Error handling**: Robust connection management and reconnection

## Deployment

### Production Build
```bash
make prod
```

This will:
1. Build the frontend into optimized static files
2. Compile the Go backend
3. Start the production server

The server serves both the game and static assets on port 8080.

### Docker (Optional)
```dockerfile
# Example Dockerfile
FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY backend/ .
RUN go build -o goblons main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/goblons .
COPY --from=builder /app/static static/
EXPOSE 8080
CMD ["./goblons"]
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## Game Balance

Current game constants (adjustable in `backend/internal/game/constants.go`):
- World size: 2000x2000 pixels
- Tick rate: 60 FPS
- Player speed: 4 pixels/frame
- Starting player size: 20 pixels
- Maximum players: 100

## License

[Add your license here]

## Credits

Inspired by doblons.io and other .io games.

