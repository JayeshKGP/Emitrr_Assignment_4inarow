package hub

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/bot"
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

	if username == bot.BotUsername {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "This username is reserved. Please choose another."})
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
		TotalDraws: playerStats.TotalDraws,
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

// PlayBot starts a game against the bot
func (h *Hub) PlayBot(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil {
		return
	}

	h.removeFromWaiting(conn)

	roomID := generateRoomID()
	r := room.NewRoom(roomID)
	h.rooms[roomID] = r

	player, _ := r.AddPlayer(conn, client.Username)
	r.AddBotPlayer(bot.BotUsername)

	client.RoomID = roomID
	client.PlayerNum = player.Number

	h.startBotGame(r)
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

	client := h.clients[conn]
	if client == nil || client.RoomID == "" {
		h.mu.Unlock()
		return
	}

	r, exists := h.rooms[client.RoomID]
	if !exists {
		h.mu.Unlock()
		return
	}

	row, winner, isDraw, err := r.MakeMove(client.PlayerNum, col)
	if err != nil {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: err.Error()})
		h.mu.Unlock()
		return
	}

	// Send move to opponent (if human)
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
		h.mu.Unlock()
		return
	}

	// If bot game and now bot's turn, make bot move
	if r.IsBotGame && r.GetCurrentTurn() == 2 && r.IsGameActive() {
		roomID := r.ID
		board := r.GetBoard()
		h.mu.Unlock()

		go h.makeBotMove(roomID, board, conn)
		return
	}

	h.mu.Unlock()
}

func (h *Hub) makeBotMove(roomID string, board [6][7]int, playerConn *websocket.Conn) {
	time.Sleep(800 * time.Millisecond)

	botCol := bot.GetBestMove(board, 2)

	h.mu.Lock()
	defer h.mu.Unlock()

	r, exists := h.rooms[roomID]
	if !exists || !r.IsGameActive() {
		return
	}

	row, winner, isDraw, err := r.MakeMove(2, botCol)
	if err != nil {
		log.Printf("Bot move error: %v", err)
		return
	}

	h.sendToConn(playerConn, models.MsgOpponentMove, models.MovePayload{Column: botCol})
	h.sendToConn(playerConn, models.MsgGameState, models.GameStatePayload{
		Board:       r.Board,
		CurrentTurn: r.CurrentTurn,
	})

	if winner != 0 || isDraw {
		h.handleGameOver(r, winner, isDraw, row, botCol)
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

	if !r.IsFull() {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "Opponent has left"})
		return
	}

	opponent := r.GetOpponent(client.PlayerNum)
	if opponent != nil && opponent.Conn != nil {
		h.sendToConn(opponent.Conn, models.MsgPlayAgainReq, nil)
	}

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

	// Check for timeouts (10 seconds) - auto-assign bot
	for i := len(h.waiting) - 1; i >= 0; i-- {
		if now.Sub(h.waiting[i].JoinedAt) > 10*time.Second {
			client := h.waiting[i].Client
			h.waiting = append(h.waiting[:i], h.waiting[i+1:]...)

			h.sendToConn(client.Conn, models.MsgBotAssigned, nil)

			roomID := generateRoomID()
			r := room.NewRoom(roomID)
			h.rooms[roomID] = r

			player, _ := r.AddPlayer(client.Conn, client.Username)
			r.AddBotPlayer(bot.BotUsername)

			client.RoomID = roomID
			client.PlayerNum = player.Number

			h.startBotGame(r)
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
	if p1.Conn != nil {
		h.sendToConn(p1.Conn, models.MsgGameStart, models.GameStartPayload{
			RoomID:        r.ID,
			Opponent:      p2.Username,
			YourNumber:    1,
			CurrentTurn:   r.CurrentTurn,
			YourScore:     p1.Score,
			OpponentScore: p2.Score,
			IsBotGame:     r.IsBotGame,
		})
	}

	// Send to player 2 (skip if bot)
	if p2.Conn != nil {
		h.sendToConn(p2.Conn, models.MsgGameStart, models.GameStartPayload{
			RoomID:        r.ID,
			Opponent:      p1.Username,
			YourNumber:    2,
			CurrentTurn:   r.CurrentTurn,
			YourScore:     p2.Score,
			OpponentScore: p1.Score,
			IsBotGame:     r.IsBotGame,
		})
	}
}

func (h *Hub) startBotGame(r *room.Room) {
	r.StartGame()
	h.notifyGameStart(r)
}

func (h *Hub) handleGameOver(r *room.Room, winner int, isDraw bool, row, col int) {
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

	if p1.Conn != nil {
		h.sendToConn(p1.Conn, models.MsgGameOver, models.GameOverPayload{
			Winner:        winner,
			WinnerName:    winnerName,
			YourScore:     p1.Score,
			OpponentScore: p2.Score,
		})
	}

	if p2.Conn != nil {
		h.sendToConn(p2.Conn, models.MsgGameOver, models.GameOverPayload{
			Winner:        winner,
			WinnerName:    winnerName,
			YourScore:     p2.Score,
			OpponentScore: p1.Score,
		})
	}

	isBotGame := r.IsBotGame
	roomID := r.ID
	go func() {
		err := h.db.SaveGame(database.GameInput{
			RoomID:          roomID,
			Player1Username: p1.Username,
			Player2Username: p2.Username,
			WinnerUsername:  winnerUsername,
			IsDraw:          isDraw,
			IsBotGame:       isBotGame,
			TotalMoves:      r.MoveCount,
			DurationSeconds: r.GetDuration(),
		})
		if err != nil {
			log.Printf("Error saving game to database: %v", err)
		} else {
			log.Printf("Game saved: %s vs %s, winner: %s, bot: %v", p1.Username, p2.Username, winnerName, isBotGame)
		}
	}()

	// Schedule room cleanup after game over (gives time for "Play Again" option)
	// For bot games, clean up immediately since there's no "Play Again" with bot
	if isBotGame {
		go h.cleanupRoom(roomID, p1.Conn)
	}
}

// cleanupRoom removes the room from memory and clears client's room reference
func (h *Hub) cleanupRoom(roomID string, playerConn *websocket.Conn) {
	// Small delay to ensure game over message is processed
	time.Sleep(100 * time.Millisecond)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Delete the room
	delete(h.rooms, roomID)

	// Clear the client's room reference
	if client, exists := h.clients[playerConn]; exists {
		client.RoomID = ""
		client.PlayerNum = 0
	}

	log.Printf("Room %s cleaned up from memory", roomID)
}

func (h *Hub) broadcastOnlineCount() {
	h.mu.RLock()
	count := len(h.clients)
	msg, _ := json.Marshal(models.WSMessage{
		Type:    models.MsgOnlineCount,
		Payload: models.OnlineCountPayload{Count: count},
	})

	// Collect client send channels while holding lock
	sendChans := make([]chan []byte, 0, len(h.clients))
	for _, client := range h.clients {
		sendChans = append(sendChans, client.Send)
	}
	h.mu.RUnlock()

	// Send to all clients (safe send that handles closed channels)
	for _, ch := range sendChans {
		func() {
			defer func() {
				recover() // Ignore panic from closed channel
			}()
			select {
			case ch <- msg:
			default:
			}
		}()
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
