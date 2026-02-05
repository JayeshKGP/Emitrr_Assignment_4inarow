package hub

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/database"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/models"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/room"
)

// Client represents a connected WebSocket client
type Client struct {
	Conn      *websocket.Conn
	PlayerID  string
	Username  string
	RoomID    string
	PlayerNum int
	Send      chan []byte
}

// WaitingClient for random matchmaking
type WaitingClient struct {
	Client    *Client
	JoinedAt  time.Time
}

// Hub manages all connections and rooms
type Hub struct {
	clients     map[*websocket.Conn]*Client
	rooms       map[string]*room.Room
	waiting     []*WaitingClient
	mu          sync.RWMutex
	broadcast   chan []byte
	db          *database.Client
}

// NewHub creates a new hub
func NewHub() *Hub {
	h := &Hub{
		clients:   make(map[*websocket.Conn]*Client),
		rooms:     make(map[string]*room.Room),
		waiting:   make([]*WaitingClient, 0),
		broadcast: make(chan []byte),
		db:        database.NewClient(),
	}
	go h.runMatchmaking()
	return h
}

// RegisterClient adds a new client
func (h *Hub) RegisterClient(conn *websocket.Conn) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := &Client{
		Conn: conn,
		Send: make(chan []byte, 256),
	}
	h.clients[conn] = client
	log.Printf("Client connected. Total online: %d", len(h.clients))
	h.broadcastOnlineCount()
	return client
}

// UnregisterClient removes a client
func (h *Hub) UnregisterClient(conn *websocket.Conn) {
	h.mu.Lock()
	client, ok := h.clients[conn]
	if !ok {
		h.mu.Unlock()
		return
	}

	// Remove from waiting queue
	h.removeFromWaiting(conn)

	// Handle room cleanup
	if client.RoomID != "" {
		if r, exists := h.rooms[client.RoomID]; exists {
			opponent := r.GetOpponent(client.PlayerNum)
			r.RemovePlayer(conn)

			// Notify opponent
			if opponent != nil && opponent.Conn != nil {
				h.sendToConn(opponent.Conn, models.MsgOpponentLeft, nil)
			}

			// Clean up empty room
			if r.IsEmpty() {
				delete(h.rooms, client.RoomID)
			}
		}
	}

	delete(h.clients, conn)
	close(client.Send)
	count := len(h.clients)
	h.mu.Unlock()
	log.Printf("Client disconnected. Total online: %d", count)
	h.broadcastOnlineCount()
}

// GetOnlineCount returns number of connected clients
func (h *Hub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Login handles user login - validates/creates player in database
func (h *Hub) Login(conn *websocket.Conn, username string) {
	client := h.clients[conn]
	if client == nil {
		return
	}

	// Get or create player in database
	playerStats, err := h.db.LoginPlayer(username)
	if err != nil {
		log.Printf("Login error for %s: %v", username, err)
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "Login failed. Please try again."})
		return
	}

	// Update client with player info
	h.mu.Lock()
	client.PlayerID = playerStats.PlayerID
	client.Username = playerStats.Username
	h.mu.Unlock()

	// Send login success with player stats
	h.sendToConn(conn, models.MsgLoginSuccess, models.LoginSuccessPayload{
		PlayerID:   playerStats.PlayerID,
		Username:   playerStats.Username,
		TotalWins:  playerStats.TotalWins,
		TotalGames: playerStats.TotalGames,
		TotalScore: playerStats.TotalScore,
	})

	log.Printf("Player logged in: %s (ID: %s)", username, playerStats.PlayerID)
}

// JoinRoom handles joining a specific room
func (h *Hub) JoinRoom(conn *websocket.Conn, roomID, username string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil {
		return
	}
	client.Username = username

	// Get or create room
	r, exists := h.rooms[roomID]
	if !exists {
		r = room.NewRoom(roomID)
		h.rooms[roomID] = r
	}

	// Check if room is full
	if r.IsFull() {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "Room is full"})
		return
	}

	// Add player to room
	player, err := r.AddPlayer(conn, username)
	if err != nil {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: err.Error()})
		return
	}

	client.RoomID = roomID
	client.PlayerNum = player.Number

	// If first player, send waiting message with room ID
	if !r.IsFull() {
		h.sendToConn(conn, models.MsgRoomCreated, models.RoomCreatedPayload{RoomID: roomID})
		h.sendToConn(conn, models.MsgWaiting, nil)
		return
	}

	// Room is full, start game
	h.startGame(r)
}

// JoinRandom handles random matchmaking
func (h *Hub) JoinRandom(conn *websocket.Conn, username string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil {
		return
	}
	client.Username = username

	// Check if already waiting
	for _, w := range h.waiting {
		if w.Client.Conn == conn {
			return
		}
	}

	// Add to waiting queue
	h.waiting = append(h.waiting, &WaitingClient{
		Client:   client,
		JoinedAt: time.Now(),
	})

	h.sendToConn(conn, models.MsgWaiting, nil)
}

// CancelSearch removes from waiting queue
func (h *Hub) CancelSearch(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeFromWaiting(conn)
}

func (h *Hub) removeFromWaiting(conn *websocket.Conn) {
	for i, w := range h.waiting {
		if w.Client.Conn == conn {
			h.waiting = append(h.waiting[:i], h.waiting[i+1:]...)
			return
		}
	}
}

// MakeMove handles a player's move
func (h *Hub) MakeMove(conn *websocket.Conn, col int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil || client.RoomID == "" {
		return
	}

	r, exists := h.rooms[client.RoomID]
	if !exists {
		return
	}

	row, winner, isDraw, err := r.MakeMove(client.PlayerNum, col)
	if err != nil {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: err.Error()})
		return
	}

	// Send move to opponent
	opponent := r.GetOpponent(client.PlayerNum)
	if opponent != nil && opponent.Conn != nil {
		h.sendToConn(opponent.Conn, models.MsgOpponentMove, models.MovePayload{Column: col})
		h.sendToConn(opponent.Conn, models.MsgGameState, models.GameStatePayload{
			Board:       r.Board,
			CurrentTurn: r.CurrentTurn,
		})
	}

	// Send state to current player
	h.sendToConn(conn, models.MsgGameState, models.GameStatePayload{
		Board:       r.Board,
		CurrentTurn: r.CurrentTurn,
	})

	// Handle game over
	if winner != 0 || isDraw {
		h.handleGameOver(r, winner, isDraw, row, col)
	}
}

// PlayAgain resets the game in the same room
func (h *Hub) PlayAgain(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil || client.RoomID == "" {
		return
	}

	r, exists := h.rooms[client.RoomID]
	if !exists {
		return
	}

	// Check if both players still in room
	if !r.IsFull() {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "Opponent has left"})
		return
	}

	// Notify opponent about play again request
	opponent := r.GetOpponent(client.PlayerNum)
	if opponent != nil && opponent.Conn != nil {
		h.sendToConn(opponent.Conn, models.MsgPlayAgainReq, nil)
	}

	// If game not active, start new game
	if !r.GameActive {
		r.StartGame()
		h.notifyGameStart(r)
	}
}

// LeaveRoom handles leaving a room
func (h *Hub) LeaveRoom(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil || client.RoomID == "" {
		return
	}

	r, exists := h.rooms[client.RoomID]
	if !exists {
		return
	}

	opponent := r.GetOpponent(client.PlayerNum)
	r.RemovePlayer(conn)

	// Notify opponent
	if opponent != nil && opponent.Conn != nil {
		h.sendToConn(opponent.Conn, models.MsgOpponentLeft, nil)
	}

	// Clean up empty room
	if r.IsEmpty() {
		delete(h.rooms, client.RoomID)
	}

	client.RoomID = ""
	client.PlayerNum = 0
}

// runMatchmaking periodically matches waiting players
func (h *Hub) runMatchmaking() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		h.processMatchmaking()
	}
}

func (h *Hub) processMatchmaking() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	// Check for timeouts (10 seconds)
	for i := len(h.waiting) - 1; i >= 0; i-- {
		if now.Sub(h.waiting[i].JoinedAt) > 10*time.Second {
			client := h.waiting[i].Client
			h.waiting = append(h.waiting[:i], h.waiting[i+1:]...)
			h.sendToConn(client.Conn, models.MsgSearchTimeout, models.ErrorPayload{
				Message: "No opponent found. Try again or create a room.",
			})
		}
	}

	// Match players
	for len(h.waiting) >= 2 {
		p1 := h.waiting[0]
		p2 := h.waiting[1]
		h.waiting = h.waiting[2:]

		// Create room
		roomID := generateRoomID()
		r := room.NewRoom(roomID)
		h.rooms[roomID] = r

		// Add players
		player1, _ := r.AddPlayer(p1.Client.Conn, p1.Client.Username)
		player2, _ := r.AddPlayer(p2.Client.Conn, p2.Client.Username)

		p1.Client.RoomID = roomID
		p1.Client.PlayerNum = player1.Number
		p2.Client.RoomID = roomID
		p2.Client.PlayerNum = player2.Number

		// Start game
		h.startGame(r)
	}
}

func (h *Hub) startGame(r *room.Room) {
	r.StartGame()
	h.notifyGameStart(r)
}

func (h *Hub) notifyGameStart(r *room.Room) {
	p1 := r.Player1
	p2 := r.Player2

	// Send to player 1
	h.sendToConn(p1.Conn, models.MsgGameStart, models.GameStartPayload{
		RoomID:        r.ID,
		Opponent:      p2.Username,
		YourNumber:    1,
		CurrentTurn:   r.CurrentTurn,
		YourScore:     p1.Score,
		OpponentScore: p2.Score,
	})

	// Send to player 2
	h.sendToConn(p2.Conn, models.MsgGameStart, models.GameStartPayload{
		RoomID:        r.ID,
		Opponent:      p1.Username,
		YourNumber:    2,
		CurrentTurn:   r.CurrentTurn,
		YourScore:     p2.Score,
		OpponentScore: p1.Score,
	})
}

func (h *Hub) handleGameOver(r *room.Room, winner int, isDraw bool, row, col int) {
	// Update scores
	r.UpdateScores(winner)

	p1 := r.Player1
	p2 := r.Player2

	var winnerName string
	var winnerUsername *string
	if isDraw {
		winnerName = "draw"
		winnerUsername = nil
	} else if winner == 1 {
		winnerName = p1.Username
		winnerUsername = &p1.Username
	} else {
		winnerName = p2.Username
		winnerUsername = &p2.Username
	}

	// Send to player 1
	h.sendToConn(p1.Conn, models.MsgGameOver, models.GameOverPayload{
		Winner:        winner,
		WinnerName:    winnerName,
		YourScore:     p1.Score,
		OpponentScore: p2.Score,
	})

	// Send to player 2
	h.sendToConn(p2.Conn, models.MsgGameOver, models.GameOverPayload{
		Winner:        winner,
		WinnerName:    winnerName,
		YourScore:     p2.Score,
		OpponentScore: p1.Score,
	})

	// Save game to database
	go func() {
		err := h.db.SaveGame(database.GameInput{
			RoomID:          r.ID,
			Player1Username: p1.Username,
			Player2Username: p2.Username,
			WinnerUsername:  winnerUsername,
			IsDraw:          isDraw,
			TotalMoves:      r.MoveCount,
			DurationSeconds: r.GetDuration(),
		})
		if err != nil {
			log.Printf("Error saving game to database: %v", err)
		} else {
			log.Printf("Game saved: %s vs %s, winner: %s", p1.Username, p2.Username, winnerName)
		}
	}()
}

func (h *Hub) broadcastOnlineCount() {
	count := len(h.clients)
	msg, _ := json.Marshal(models.WSMessage{
		Type:    models.MsgOnlineCount,
		Payload: models.OnlineCountPayload{Count: count},
	})

	for _, client := range h.clients {
		select {
		case client.Send <- msg:
		default:
		}
	}
}

func (h *Hub) sendToConn(conn *websocket.Conn, msgType string, payload interface{}) {
	msg, err := json.Marshal(models.WSMessage{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	client := h.clients[conn]
	if client != nil {
		select {
		case client.Send <- msg:
		default:
			log.Printf("Client send buffer full")
		}
	}
}

func generateRoomID() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
