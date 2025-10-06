// Game constants (should match backend)
const WorldWidth = 2000.0;
const WorldHeight = 2000.0;

class GameClient {
  constructor() {
    this.canvas = document.getElementById('game');
    this.ctx = this.canvas.getContext('2d');
    this.socket = null;
    this.myPlayerId = null;
    this.gameState = {
      players: [],
      items: [],
      bullets: [],
      myPlayer: null
    };
    this.input = { 
      type: 'input',
      up: false, 
      down: false, 
      left: false, 
      right: false,
      shootLeft: false,
      shootRight: false,
      upgradeCannons: false,
      downgradeCannons: false,
      mouse: { x: 0, y: 0 }
    };
    
    // Ship physics properties for client-side prediction

    this.shipPhysics = {
      angle: 0,           // Current facing direction (radians)
      velocity: { x: 0, y: 0 },  // Current velocity
      acceleration: 1000,   // Forward acceleration (matches server)
      deceleration: 0.97,  // Drag/friction factor (matches server)
      turnSpeed: 0.07,      // How fast the ship turns (matches server)
      maxSpeed: 4.0        // Maximum speed (matches server)
    };
    
    this.camera = { x: 0, y: 0, targetX: 0, targetY: 0 };
    this.isConnected = false;
    this.screenWidth = window.innerWidth;
    this.screenHeight = window.innerHeight;
    
    // Client-side prediction
    this.predictedPlayerPos = { x: 0, y: 0 };
    this.lastPredictionUpdate = Date.now();
    
    this.resizeCanvas();
    this.init();
  }

  init() {
    this.setupEventListeners();
    this.connect();
    this.startGameLoop();
  }

  resizeCanvas() {
    const dpr = window.devicePixelRatio || 1;
    const displayWidth = window.innerWidth;
    const displayHeight = window.innerHeight;
    
    // Store the current transform matrix
    this.ctx.save();
    
    // Reset the current transform
    this.ctx.setTransform(1, 0, 0, 1, 0, 0);
    
    this.canvas.width = displayWidth * dpr;
    this.canvas.height = displayHeight * dpr;
    
    this.canvas.style.width = displayWidth + 'px';
    this.canvas.style.height = displayHeight + 'px';
    
    // Scale the context for high DPI displays
    this.ctx.scale(dpr, dpr);
    
    // Store screen dimensions for rendering calculations
    this.screenWidth = displayWidth;
    this.screenHeight = displayHeight;
  }

  setupEventListeners() {
    // Keyboard input
    document.addEventListener('keydown', (e) => {
      this.handleKeyDown(e);
    });
    
    document.addEventListener('keyup', (e) => {
      this.handleKeyUp(e);
    });
    
    // Mouse input
    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      this.input.mouse.x = e.clientX - rect.left;
      this.input.mouse.y = e.clientY - rect.top;
    });

    // Handle window resize
    window.addEventListener('resize', () => {
      this.resizeCanvas();
    });

    // Handle fullscreen toggle
    document.addEventListener('keydown', (e) => {
      if (e.key === 'F11') {
        e.preventDefault();
        this.toggleFullscreen();
      }
    });
  }

  toggleFullscreen() {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen().catch(console.error);
    } else {
      document.exitFullscreen().catch(console.error);
    }
  }

  connect() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    this.socket = new WebSocket(`${protocol}//${location.host}/ws`);
    
    this.socket.onopen = () => {
      console.log('Connected to server');
      this.isConnected = true;
      this.updateConnectionStatus(true);
    };
    
    this.socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      this.handleMessage(data);
    };
    
    this.socket.onclose = () => {
      console.log('Disconnected from server');
      this.isConnected = false;
      this.updateConnectionStatus(false);
      // Try to reconnect after 3 seconds
      setTimeout(() => this.connect(), 3000);
    };
    
    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }

  handleMessage(data) {
    switch (data.type) {
      case 'snapshot':
        this.gameState.players = data.players || [];
        this.gameState.items = data.items || [];
        this.gameState.bullets = data.bullets || [];
        
        // Try to find our player by keeping track of the last known position
        if (!this.gameState.myPlayer && this.gameState.players.length > 0) {
          // For now, assume the first player is ours when we first connect
          this.gameState.myPlayer = this.gameState.players[0];
          this.myPlayerId = this.gameState.myPlayer.id;
          
          // Initialize predicted position and ship physics with server data
          this.predictedPlayerPos.x = this.gameState.myPlayer.x;
          this.predictedPlayerPos.y = this.gameState.myPlayer.y;
          
          if (this.gameState.myPlayer.angle !== undefined) {
            this.shipPhysics.angle = this.gameState.myPlayer.angle;
          }
          
          // Initialize velocity from server
          this.shipPhysics.velocity.x = this.gameState.myPlayer.velX || 0;
          this.shipPhysics.velocity.y = this.gameState.myPlayer.velY || 0;
        } else if (this.myPlayerId) {
          // Find our player by ID
          const serverPlayer = this.gameState.players.find(p => p.id === this.myPlayerId);
          if (serverPlayer) {
            this.gameState.myPlayer = serverPlayer;
            
            // Sync angle with server
            if (serverPlayer.angle !== undefined) {
              this.shipPhysics.angle = serverPlayer.angle;
            }
            
            // Reconcile predicted position with server position
            const serverPos = { x: serverPlayer.x, y: serverPlayer.y };
            const distance = Math.sqrt(
              Math.pow(this.predictedPlayerPos.x - serverPos.x, 2) + 
              Math.pow(this.predictedPlayerPos.y - serverPos.y, 2)
            );
            
            // If prediction is too far off, snap to server position
            if (distance > 25) {
              this.predictedPlayerPos.x = serverPos.x;
              this.predictedPlayerPos.y = serverPos.y;
              // Also sync velocity to prevent further drift
              this.shipPhysics.velocity.x = serverPlayer.velX || 0;
              this.shipPhysics.velocity.y = serverPlayer.velY || 0;
            } else if (distance > 5) {
              // Gradually correct prediction towards server position
              const correctionFactor = 0.15;
              this.predictedPlayerPos.x += (serverPos.x - this.predictedPlayerPos.x) * correctionFactor;
              this.predictedPlayerPos.y += (serverPos.y - this.predictedPlayerPos.y) * correctionFactor;
            }
          }
        }
        break;
        
      case 'playerJoined':
        console.log(`Player ${data.playerId} joined the game`);
        break;
        
      case 'playerLeft':
        console.log(`Player ${data.playerId} left the game`);
        break;
        
      case 'gameEvent':
        this.handleGameEvent(data);
        break;
        
      default:
        console.log('Unknown message type:', data.type);
    }
  }

  handleGameEvent(data) {
    switch (data.eventType) {
      case 'playerKilled':
        if (data.victimId === this.myPlayerId) {
          console.log('You were killed!');
          // Could show death screen or respawn message
        }
        break;
      case 'itemCollected':
        // Could add visual effects for item collection
        break;
    }
  }

  setupEventListeners() {
    // Keyboard input
    document.addEventListener('keydown', (e) => {
      this.handleKeyDown(e);
    });
    
    document.addEventListener('keyup', (e) => {
      this.handleKeyUp(e);
    });
    
    // Mouse input
    this.canvas.addEventListener('mousemove', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      this.input.mouse.x = e.clientX - rect.left;
      this.input.mouse.y = e.clientY - rect.top;
    });
    
    // Prevent context menu on right click
    this.canvas.addEventListener('contextmenu', (e) => {
      e.preventDefault();
    });
  }

  handleKeyDown(e) {
    let inputChanged = false;
    
    if (e.key === 'w' || e.key === 'W' || e.key === 'ArrowUp') {
      if (!this.input.up) {
        this.input.up = true;
        inputChanged = true;
      }
    }
    if (e.key === 's' || e.key === 'S' || e.key === 'ArrowDown') {
      if (!this.input.down) {
        this.input.down = true;
        inputChanged = true;
      }
    }
    if (e.key === 'a' || e.key === 'A' || e.key === 'ArrowLeft') {
      if (!this.input.left) {
        this.input.left = true;
        inputChanged = true;
      }
    }
    if (e.key === 'd' || e.key === 'D' || e.key === 'ArrowRight') {
      if (!this.input.right) {
        this.input.right = true;
        inputChanged = true;
      }
    }
    if (e.key === 'q' || e.key === 'Q') {
      if (!this.input.shootLeft) {
        this.input.shootLeft = true;
        inputChanged = true;
      }
    }
    if (e.key === 'e' || e.key === 'E') {
      if (!this.input.shootRight) {
        this.input.shootRight = true;
        inputChanged = true;
      }
    }
    if (e.key === '=' || e.key === '+') {
      if (!this.input.upgradeCannons) {
        this.input.upgradeCannons = true;
        inputChanged = true;
      }
    }
    if (e.key === '-' || e.key === '_') {
      if (!this.input.downgradeCannons) {
        this.input.downgradeCannons = true;
        inputChanged = true;
      }
    }
    
    if (inputChanged) {
      this.sendInput();
    }
  }

  handleKeyUp(e) {
    let inputChanged = false;
    
    if (e.key === 'w' || e.key === 'W' || e.key === 'ArrowUp') {
      if (this.input.up) {
        this.input.up = false;
        inputChanged = true;
      }
    }
    if (e.key === 's' || e.key === 'S' || e.key === 'ArrowDown') {
      if (this.input.down) {
        this.input.down = false;
        inputChanged = true;
      }
    }
    if (e.key === 'a' || e.key === 'A' || e.key === 'ArrowLeft') {
      if (this.input.left) {
        this.input.left = false;
        inputChanged = true;
      }
    }
    if (e.key === 'd' || e.key === 'D' || e.key === 'ArrowRight') {
      if (this.input.right) {
        this.input.right = false;
        inputChanged = true;
      }
    }
    if (e.key === 'q' || e.key === 'Q') {
      if (this.input.shootLeft) {
        this.input.shootLeft = false;
        inputChanged = true;
      }
    }
    if (e.key === 'e' || e.key === 'E') {
      if (this.input.shootRight) {
        this.input.shootRight = false;
        inputChanged = true;
      }
    }
    if (e.key === '=' || e.key === '+') {
      if (this.input.upgradeCannons) {
        this.input.upgradeCannons = false;
        inputChanged = true;
      }
    }
    if (e.key === '-' || e.key === '_') {
      if (this.input.downgradeCannons) {
        this.input.downgradeCannons = false;
        inputChanged = true;
      }
    }
    
    if (inputChanged) {
      this.sendInput();
    }
  }

  sendInput() {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(this.input));
    }
  }

  updateClientPrediction() {
    if (!this.gameState.myPlayer) return;
    
    const currentTime = Date.now();
    const deltaTime = Math.min((currentTime - this.lastPredictionUpdate) / 1000, 1/30); // Cap deltaTime
    this.lastPredictionUpdate = currentTime;
    
    // Ship physics simulation (matching server)
    const physics = this.shipPhysics;
    
    // Handle thrust (W/S keys) - this affects speed, not direction
    let thrustForce = 0;
    if (this.input.up) {
      thrustForce = physics.acceleration;
    }
    if (this.input.down) {
      thrustForce = -physics.acceleration * 0.5; // Reverse is weaker
    }
    
    // Apply thrust in the direction the ship is facing
    if (thrustForce !== 0) {
      const thrustX = Math.cos(physics.angle) * thrustForce;
      const thrustY = Math.sin(physics.angle) * thrustForce;
      physics.velocity.x += thrustX;
      physics.velocity.y += thrustY;
    }
    
    // Calculate current speed for turn scaling (matching server logic)
    const speed = Math.min(Math.sqrt(physics.velocity.x * physics.velocity.x + physics.velocity.y * physics.velocity.y), physics.maxSpeed);
    
    // Scale turn speed based on current speed (matching server logic)
    let turnFactor = speed / physics.maxSpeed;
    const scaledTurnSpeed = physics.turnSpeed * turnFactor;
    
    // Handle turning (A/D keys) with speed-based scaling
    if (this.input.left) {
      physics.angle -= scaledTurnSpeed;
    }
    if (this.input.right) {
      physics.angle += scaledTurnSpeed;
    }
    
    // Apply drag/deceleration
    physics.velocity.x *= physics.deceleration;
    physics.velocity.y *= physics.deceleration;
    
    // Limit maximum speed
    const newSpeed = Math.sqrt(physics.velocity.x * physics.velocity.x + physics.velocity.y * physics.velocity.y);
    if (newSpeed > physics.maxSpeed) {
      const speedRatio = physics.maxSpeed / newSpeed;
      physics.velocity.x *= speedRatio;
      physics.velocity.y *= speedRatio;
    }
    
    // Update predicted position with more conservative movement
    const moveX = physics.velocity.x * deltaTime * 30; // Reduced multiplier
    const moveY = physics.velocity.y * deltaTime * 30;
    
    this.predictedPlayerPos.x += moveX;
    this.predictedPlayerPos.y += moveY;
    
    // Keep within world bounds
    if (this.predictedPlayerPos.x <= 10) {
      this.predictedPlayerPos.x = 10;
      physics.velocity.x = Math.max(0, physics.velocity.x); // Stop negative velocity
    }
    if (this.predictedPlayerPos.x >= WorldWidth - 10) {
      this.predictedPlayerPos.x = WorldWidth - 10;
      physics.velocity.x = Math.min(0, physics.velocity.x); // Stop positive velocity
    }
    if (this.predictedPlayerPos.y <= 10) {
      this.predictedPlayerPos.y = 10;
      physics.velocity.y = Math.max(0, physics.velocity.y);
    }
    if (this.predictedPlayerPos.y >= WorldHeight - 10) {
      this.predictedPlayerPos.y = WorldHeight - 10;
      physics.velocity.y = Math.min(0, physics.velocity.y);
    }
  }

  updateCamera() {
    if (this.gameState.myPlayer) {
      // Use server position for camera to avoid jitter
      // this.camera.targetX = this.predictedPlayerPos.x - this.screenWidth / 2;
      // this.camera.targetY = this.predictedPlayerPos.y - this.screenHeight / 2;
      this.camera.targetX = this.gameState.myPlayer.x - this.screenWidth / 2;
      this.camera.targetY = this.gameState.myPlayer.y - this.screenHeight / 2;
      
      // Smooth camera movement
      const cameraLerpFactor = 1;
      this.camera.x += (this.camera.targetX - this.camera.x) * cameraLerpFactor;
      this.camera.y += (this.camera.targetY - this.camera.y) * cameraLerpFactor;
    }
  }

  render() {
    // Clear canvas
    this.ctx.fillStyle = '#adb5db';
    this.ctx.fillRect(0, 0, this.screenWidth, this.screenHeight);
    
    if (!this.isConnected) {
      this.ctx.fillStyle = '#ff6b6b';
      this.ctx.font = '24px Arial';
      this.ctx.textAlign = 'center';
      this.ctx.fillText('Connecting...', this.screenWidth / 2, this.screenHeight / 2);
      return;
    }

    this.updateClientPrediction();
    this.updateCamera();

    // Draw world grid
    this.drawGrid();
    
    // Draw map border
    this.drawMapBorder();
    
    // Draw items
    this.gameState.items.forEach(item => {
      this.drawItem(item);
    });
    
    // Draw bullets
    this.gameState.bullets.forEach(bullet => {
      this.drawBullet(bullet);
    });
    
    // Draw players
    this.gameState.players.forEach(player => {
      this.drawPlayer(player);
    });
    
    // Draw UI
    this.drawUI();
  }

  drawGrid() {
    const gridSize = 25;
    this.ctx.strokeStyle = '#9393a3ff';
    this.ctx.lineWidth = 1;
    
    const startX = Math.floor(this.camera.x / gridSize) * gridSize;
    const startY = Math.floor(this.camera.y / gridSize) * gridSize;
    
    for (let x = startX; x < this.camera.x + this.screenWidth; x += gridSize) {
      this.ctx.beginPath();
      this.ctx.moveTo(x - this.camera.x, 0);
      this.ctx.lineTo(x - this.camera.x, this.screenHeight);
      this.ctx.stroke();
    }
    
    for (let y = startY; y < this.camera.y + this.screenHeight; y += gridSize) {
      this.ctx.beginPath();
      this.ctx.moveTo(0, y - this.camera.y);
      this.ctx.lineTo(this.screenWidth, y - this.camera.y);
      this.ctx.stroke();
    }
  }

  drawMapBorder() {
    const worldWidth = 2000;
    const worldHeight = 2000;
    
    // Convert world coordinates to screen coordinates
    const borderLeft = 0 - this.camera.x;
    const borderTop = 0 - this.camera.y;
    const borderRight = worldWidth - this.camera.x;
    const borderBottom = worldHeight - this.camera.y;
    
    // Only draw border segments that are visible on screen
    this.ctx.strokeStyle = '#333'; 
    this.ctx.lineWidth = 4;
    
    this.ctx.beginPath();
    
    // Top border
    if (borderTop >= -4 && borderTop <= this.screenHeight + 4) {
      this.ctx.moveTo(Math.max(0, borderLeft), borderTop);
      this.ctx.lineTo(Math.min(this.screenWidth, borderRight), borderTop);
    }
    
    // Bottom border
    if (borderBottom >= -4 && borderBottom <= this.screenHeight + 4) {
      this.ctx.moveTo(Math.max(0, borderLeft), borderBottom);
      this.ctx.lineTo(Math.min(this.screenWidth, borderRight), borderBottom);
    }
    
    // Left border
    if (borderLeft >= -4 && borderLeft <= this.screenWidth + 4) {
      this.ctx.moveTo(borderLeft, Math.max(0, borderTop));
      this.ctx.lineTo(borderLeft, Math.min(this.screenHeight, borderBottom));
    }
    
    // Right border
    if (borderRight >= -4 && borderRight <= this.screenWidth + 4) {
      this.ctx.moveTo(borderRight, Math.max(0, borderTop));
      this.ctx.lineTo(borderRight, Math.min(this.screenHeight, borderBottom));
    }
    
    this.ctx.stroke();
  }

drawPlayer(player) {
  const ctx = this.ctx;
  const screenX = player.x - this.camera.x;
  const screenY = player.y - this.camera.y;

  const size = player.size || 20;
  const color = player.color || '#d9534f';
  const angle = player.angle || 0;

  // --- Base ship dimensions ---
  const bowLength = size * 0.4;
  const baseShaftLength = (player.shipLength || size * 1.2) * 0.5;
  const rearLength = size * 0.3;
  const shaftWidth = player.shipWidth || size * 0.6;

  // --- Guns settings ---
  const gunCount = player.cannonCount || 2; // Total guns per side from server
  const gunLength = size * 0.35;
  const gunWidth = size * 0.2;

  // --- Determine number of guns per side ---
  const leftGunCount = gunCount;  // Server sends cannons per side
  const rightGunCount = gunCount; // Same number on both sides

  // --- Ship length adjustment ---
  // Add extra shaft length if more than 1 gun per side
  const spacing = gunLength * 1.5;
  const extraShaftLength = Math.max(leftGunCount, rightGunCount) > 1 ? spacing * (Math.max(leftGunCount, rightGunCount) - 1) : 0;
  const shaftLength = baseShaftLength + extraShaftLength;
  const totalRearLength = rearLength;

  ctx.save();
  ctx.translate(screenX, screenY);
  ctx.rotate(angle);

  ctx.fillStyle = color;
  ctx.strokeStyle = '#444';
  ctx.lineWidth = 3;

  // --- Draw main hull ---
  ctx.beginPath();
  ctx.moveTo(shaftLength / 2 + bowLength, 0); // bow tip

  ctx.quadraticCurveTo(
    shaftLength / 2 + bowLength * 0.3,
    shaftWidth / 2,
    shaftLength / 2,
    shaftWidth / 2
  );
  ctx.lineTo(-shaftLength / 2, shaftWidth / 2);
  ctx.lineTo(-shaftLength / 2 - totalRearLength, shaftWidth / 2 * 0.5);
  ctx.lineTo(-shaftLength / 2 - totalRearLength, -shaftWidth / 2 * 0.5);
  ctx.lineTo(-shaftLength / 2, -shaftWidth / 2);
  ctx.lineTo(shaftLength / 2, -shaftWidth / 2);
  ctx.quadraticCurveTo(
    shaftLength / 2 + bowLength * 0.3,
    -shaftWidth / 2,
    shaftLength / 2 + bowLength,
    0
  );
  ctx.closePath();
  ctx.fill();
  ctx.stroke();

  // --- Center circle ---
  ctx.beginPath();
  ctx.arc(0, 0, shaftWidth * 0.2, 0, Math.PI * 2);
  ctx.fillStyle = '#444';
  ctx.fill();
  ctx.strokeStyle = '#444';
  ctx.stroke();

  // --- Draw guns evenly spaced along ship ---
  ctx.fillStyle = '#444';

  const leftGunSpacing = shaftLength / (leftGunCount + 1);
  const rightGunSpacing = shaftLength / (rightGunCount + 1);

  for (let i = 0; i < leftGunCount; i++) {
    const x = -shaftLength / 2 + (i + 1) * leftGunSpacing - gunLength / 2;
    const y = -shaftWidth / 2 - gunWidth; // left side (negative Y)
    ctx.fillRect(x, y, gunLength, gunWidth);
  }

  for (let i = 0; i < rightGunCount; i++) {
    const x = -shaftLength / 2 + (i + 1) * rightGunSpacing - gunLength / 2;
    const y = shaftWidth / 2; // right side (positive Y)
    ctx.fillRect(x, y, gunLength, gunWidth);
  }

  ctx.restore();
}

  drawItem(item) {
    const screenX = item.x - this.camera.x;
    const screenY = item.y - this.camera.y;
    
    // Skip if item is off screen
    if (screenX < -20 || screenX > this.screenWidth + 20 ||
        screenY < -20 || screenY > this.screenHeight + 20) {
      return;
    }
    
    let color = '#FFD700'; // Default gold
    let size = 8;
    let shape = 'circle';
    
    switch (item.type) {
      case 'coin':
        color = '#FFD700';
        size = 6;
        break;
      case 'health_pack':
        color = '#FF6B6B';
        size = 10;
        shape = 'cross';
        break;
      case 'food':
        color = '#4ECDC4';
        size = 4;
        break;
      case 'size_boost':
        color = '#96CEB4';
        size = 12;
        break;
      case 'speed_boost':
        color = '#FFEAA7';
        size = 10;
        shape = 'diamond';
        break;
      case 'score_multiplier':
        color = '#DDA0DD';
        size = 14;
        shape = 'star';
        break;
    }
    
    this.ctx.fillStyle = color;
    
    if (shape === 'circle') {
      this.ctx.beginPath();
      this.ctx.arc(screenX, screenY, size, 0, Math.PI * 2);
      this.ctx.fill();
    } else if (shape === 'cross') {
      // Draw a cross for health packs
      this.ctx.fillRect(screenX - size/2, screenY - size/6, size, size/3);
      this.ctx.fillRect(screenX - size/6, screenY - size/2, size/3, size);
    } else if (shape === 'diamond') {
      // Draw a diamond for speed boost
      this.ctx.beginPath();
      this.ctx.moveTo(screenX, screenY - size);
      this.ctx.lineTo(screenX + size, screenY);
      this.ctx.lineTo(screenX, screenY + size);
      this.ctx.lineTo(screenX - size, screenY);
      this.ctx.closePath();
      this.ctx.fill();
    } else if (shape === 'star') {
      // Draw a simple star for score multiplier
      this.ctx.beginPath();
      for (let i = 0; i < 5; i++) {
        const angle = (i * 2 * Math.PI) / 5 - Math.PI / 2;
        const x = screenX + Math.cos(angle) * size;
        const y = screenY + Math.sin(angle) * size;
        if (i === 0) this.ctx.moveTo(x, y);
        else this.ctx.lineTo(x, y);
      }
      this.ctx.closePath();
      this.ctx.fill();
    }
    
    // Add sparkle effect
    this.ctx.strokeStyle = '#ffffff';
    this.ctx.lineWidth = 1;
    this.ctx.stroke();
  }

  drawBullet(bullet) {
    const screenX = bullet.x - this.camera.x;
    const screenY = bullet.y - this.camera.y;
    
    // Only draw if bullet is on screen
    if (screenX < -50 || screenX > this.screenWidth + 50 || 
        screenY < -50 || screenY > this.screenHeight + 50) {
      return;
    }
    
    this.ctx.save();
    this.ctx.translate(screenX, screenY);
    
    // Draw bullet as a bright orange/yellow circle
    this.ctx.beginPath();
    this.ctx.arc(0, 0, bullet.size, 0, Math.PI * 2);
    this.ctx.fillStyle = '#484848ff'; // Gold color for bullets
    this.ctx.fill();
    
    // Add a bright outline
    this.ctx.strokeStyle = '#2a2a2aff'; // Orange outline
    this.ctx.lineWidth = 1;
    this.ctx.stroke();
    
    this.ctx.restore();
  }

  drawUI() {
    // Semi-transparent background for UI elements
    this.ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
    this.ctx.fillRect(10, 10, 200, 100);
    
    if (this.gameState.myPlayer) {
      // Player info
      this.ctx.fillStyle = '#ffffff';
      this.ctx.font = '18px Arial';
      this.ctx.textAlign = 'left';
      this.ctx.fillText(`${this.gameState.myPlayer.name}`, 20, 35);
      this.ctx.fillText(`Score: ${this.gameState.myPlayer.score || 0}`, 20, 55);
      this.ctx.fillText(`Cannons: ${this.gameState.myPlayer.cannonCount || 2}/side`, 20, 75);
      this.ctx.fillText(`Players: ${this.gameState.players.length}`, 20, 95);
    }
    
    // Mini leaderboard in top right
    this.drawLeaderboard();
    
    // Draw controls help
    this.drawControls();
    
    // Draw minimap
    this.drawMinimap();
  }

  drawLeaderboard() {
    const sortedPlayers = [...this.gameState.players]
      .sort((a, b) => (b.score || 0) - (a.score || 0))
      .slice(0, 5); // Top 5 players
    
    if (sortedPlayers.length === 0) return;
    
    const leaderboardWidth = 180;
    const leaderboardHeight = 30 + sortedPlayers.length * 25;
    const x = this.screenWidth - leaderboardWidth - 10;
    const y = 10;
    
    // Background
    this.ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
    this.ctx.fillRect(x, y, leaderboardWidth, leaderboardHeight);
    
    // Title
    this.ctx.fillStyle = '#4ECDC4';
    this.ctx.font = 'bold 16px Arial';
    this.ctx.textAlign = 'left';
    this.ctx.fillText('Leaderboard', x + 10, y + 20);
    
    // Players
    this.ctx.font = '14px Arial';
    sortedPlayers.forEach((player, index) => {
      const isMe = player.id === this.myPlayerId;
      this.ctx.fillStyle = isMe ? '#FFD700' : '#ffffff';
      
      const rank = index + 1;
      const name = player.name || `Player ${player.id}`;
      const score = player.score || 0;
      
      this.ctx.fillText(`${rank}. ${name}`, x + 10, y + 45 + index * 25);
      this.ctx.textAlign = 'right';
      this.ctx.fillText(`${score}`, x + leaderboardWidth - 10, y + 45 + index * 25);
      this.ctx.textAlign = 'left';
    });
  }

  drawControls() {
    const controlsWidth = 200;
    const controlsHeight = 150;
    const x = 10;
    const y = this.screenHeight - controlsHeight - 10;
    
    // Background
    this.ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
    this.ctx.fillRect(x, y, controlsWidth, controlsHeight);
    
    // Controls text
    this.ctx.fillStyle = '#ffffff';
    this.ctx.font = '14px Arial';
    this.ctx.textAlign = 'left';
    
    this.ctx.fillText('CONTROLS:', x + 10, y + 20);
    this.ctx.font = '12px Arial';
    this.ctx.fillText('WASD: Move', x + 10, y + 40);
    this.ctx.fillText('Q: Fire Left Cannons', x + 10, y + 55);
    this.ctx.fillText('E: Fire Right Cannons', x + 10, y + 70);
    this.ctx.fillText('+: Add Cannons', x + 10, y + 85);
    this.ctx.fillText('-: Remove Cannons', x + 10, y + 100);
    this.ctx.fillText('Ships turn faster', x + 10, y + 120);
    this.ctx.fillText('when moving!', x + 10, y + 135);
  }

  drawMinimap() {
    const minimapSize = 120;
    const minimapX = this.screenWidth - minimapSize - 20;
    const minimapY = this.screenHeight - minimapSize - 20;
    
    // Background
    this.ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
    this.ctx.fillRect(minimapX, minimapY, minimapSize, minimapSize);
    
    // Border
    this.ctx.strokeStyle = '#4ECDC4';
    this.ctx.lineWidth = 2;
    this.ctx.strokeRect(minimapX, minimapY, minimapSize, minimapSize);
    
    // Scale factors
    const scaleX = minimapSize / WorldWidth;
    const scaleY = minimapSize / WorldHeight;
    
    // Draw players as dots
    this.gameState.players.forEach(player => {
      const dotX = minimapX + (player.x * scaleX);
      const dotY = minimapY + (player.y * scaleY);
      const dotSize = Math.max(2, player.size * scaleX * 0.5);
      
      this.ctx.fillStyle = player.id === this.myPlayerId ? '#FFD700' : '#4ECDC4';
      this.ctx.beginPath();
      this.ctx.arc(dotX, dotY, dotSize, 0, Math.PI * 2);
      this.ctx.fill();
    });
    
    // Draw items as smaller dots
    this.ctx.fillStyle = 'rgba(255, 215, 0, 0.6)';
    this.gameState.items.forEach(item => {
      const dotX = minimapX + (item.x * scaleX);
      const dotY = minimapY + (item.y * scaleY);
      
      this.ctx.beginPath();
      this.ctx.arc(dotX, dotY, 1, 0, Math.PI * 2);
      this.ctx.fill();
    });
  }

  updateConnectionStatus(connected) {
    const statusElement = document.querySelector('#connectionStatus');
    const indicator = statusElement.querySelector('.status-indicator');
    const text = statusElement.querySelector('span:last-child');
    
    if (connected) {
      indicator.className = 'status-indicator connected';
      text.textContent = 'Connected';
    } else {
      indicator.className = 'status-indicator disconnected';
      text.textContent = 'Reconnecting...';
    }
  }

  startGameLoop() {
    const gameLoop = () => {
      this.render();
      requestAnimationFrame(gameLoop);
    };
    gameLoop();
  }
}

// Start the game when the page loads
window.addEventListener('load', () => {
  new GameClient();
});

// Game client is now initialized in the GameClient class above
