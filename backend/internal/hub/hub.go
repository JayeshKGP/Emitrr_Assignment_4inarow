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
	"github.com/jayeshdeshmukh/connect-four-backend/internal/kafka"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/models"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/room"
)

type Client struct {
	Conn      *websocket.Conn
	PlayerID  string
	Username  string
	RoomID    string
	PlayerNum int
	Send      chan []byte
}

type WaitingClient struct {
	Client   *Client
	JoinedAt time.Time
}

type Hub struct {
	clients       map[*websocket.Conn]*Client
	rooms         map[string]*room.Room
	waiting       []*WaitingClient
	mu            sync.RWMutex
	broadcast     chan []byte
	db            *database.Client
	kafkaProducer *kafka.Producer
	kafkaConsumer *kafka.Consumer
}

func NewHub() *Hub {
	h := &Hub{
		clients:       make(map[*websocket.Conn]*Client),
		rooms:         make(map[string]*room.Room),
		waiting:       make([]*WaitingClient, 0),
		broadcast:     make(chan []byte),
		db:            database.NewClient(),
		kafkaConsumer: kafka.NewConsumerWithoutKafka(),
	}

	go func() {
		producer := kafka.NewProducer()
		consumer := kafka.NewConsumer()

		h.mu.Lock()
		h.kafkaProducer = producer
		if consumer.IsEnabled() {
			h.kafkaConsumer = consumer
		}
		h.mu.Unlock()
	}()

	go h.runMatchmaking()
	return h
}

func (h *Hub) Close() {
	if h.kafkaProducer != nil {
		h.kafkaProducer.Close()
	}
	if h.kafkaConsumer != nil {
		h.kafkaConsumer.Close()
	}
}

func (h *Hub) GetMetrics() kafka.CalculatedMetrics {
	h.mu.RLock()
	consumer := h.kafkaConsumer
	h.mu.RUnlock()

	if consumer == nil {
		return kafka.CalculatedMetrics{}
	}
	return consumer.GetMetrics()
}

func (h *Hub) RegisterClient(conn *websocket.Conn) *Client {
	h.mu.Lock()
	client := &Client{
		Conn: conn,
		Send: make(chan []byte, 256),
	}
	h.clients[conn] = client
	h.mu.Unlock()

	h.broadcastOnlineCount()
	return client
}

func (h *Hub) UnregisterClient(conn *websocket.Conn) {
	h.mu.Lock()
	client, ok := h.clients[conn]
	if !ok {
		h.mu.Unlock()
		return
	}

	h.removeFromWaiting(conn)

	if client.RoomID != "" {
		if r, exists := h.rooms[client.RoomID]; exists {
			// If game was active, end it and update metrics
			if r.GameActive {
				h.endGameDueToLeave(r)
			}

			opponent := r.GetOpponent(client.PlayerNum)
			r.RemovePlayer(conn)

			if opponent != nil && opponent.Conn != nil {
				h.sendToConn(opponent.Conn, models.MsgOpponentLeft, nil)
			}

			if r.IsEmpty() {
				delete(h.rooms, client.RoomID)
			}
		}
	}

	delete(h.clients, conn)
	close(client.Send)
	h.mu.Unlock()

	h.broadcastOnlineCount()
}

func (h *Hub) GetOnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Login(conn *websocket.Conn, username string) {
	client := h.clients[conn]
	if client == nil {
		return
	}

	if username == bot.BotUsername {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "This username is reserved. Please choose another."})
		return
	}

	playerStats, err := h.db.LoginPlayer(username)
	if err != nil {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "Login failed. Please try again."})
		return
	}

	h.mu.Lock()
	client.PlayerID = playerStats.PlayerID
	client.Username = playerStats.Username
	h.mu.Unlock()

	h.sendToConn(conn, models.MsgLoginSuccess, models.LoginSuccessPayload{
		PlayerID:   playerStats.PlayerID,
		Username:   playerStats.Username,
		TotalWins:  playerStats.TotalWins,
		TotalDraws: playerStats.TotalDraws,
		TotalGames: playerStats.TotalGames,
		TotalScore: playerStats.TotalScore,
	})
}

func (h *Hub) JoinRoom(conn *websocket.Conn, roomID, username string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil {
		return
	}
	client.Username = username

	r, exists := h.rooms[roomID]
	if !exists {
		r = room.NewRoom(roomID)
		h.rooms[roomID] = r
	}

	if r.IsFull() {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: "Room is full"})
		return
	}

	player, err := r.AddPlayer(conn, username)
	if err != nil {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: err.Error()})
		return
	}

	client.RoomID = roomID
	client.PlayerNum = player.Number

	if !r.IsFull() {
		h.sendToConn(conn, models.MsgRoomCreated, models.RoomCreatedPayload{RoomID: roomID})
		h.sendToConn(conn, models.MsgWaiting, nil)
		return
	}

	h.startGame(r)
}

func (h *Hub) JoinRandom(conn *websocket.Conn, username string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil {
		return
	}
	client.Username = username

	for _, w := range h.waiting {
		if w.Client.Conn == conn {
			return
		}
	}

	h.waiting = append(h.waiting, &WaitingClient{
		Client:   client,
		JoinedAt: time.Now(),
	})

	h.sendToConn(conn, models.MsgWaiting, nil)
}

func (h *Hub) CancelSearch(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeFromWaiting(conn)
}

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

	_, winner, isDraw, winningCells, err := r.MakeMove(client.PlayerNum, col)
	if err != nil {
		h.sendToConn(conn, models.MsgError, models.ErrorPayload{Message: err.Error()})
		h.mu.Unlock()
		return
	}

	opponent := r.GetOpponent(client.PlayerNum)
	if opponent != nil && opponent.Conn != nil {
		h.sendToConn(opponent.Conn, models.MsgOpponentMove, models.MovePayload{Column: col})
		h.sendToConn(opponent.Conn, models.MsgGameState, models.GameStatePayload{
			Board:       r.Board,
			CurrentTurn: r.CurrentTurn,
		})
	}

	h.sendToConn(conn, models.MsgGameState, models.GameStatePayload{
		Board:       r.Board,
		CurrentTurn: r.CurrentTurn,
	})

	if winner != 0 || isDraw {
		h.handleGameOver(r, winner, isDraw, winningCells)
		h.mu.Unlock()
		return
	}

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

	_, winner, isDraw, winningCells, err := r.MakeMove(2, botCol)
	if err != nil {
		return
	}

	h.sendToConn(playerConn, models.MsgOpponentMove, models.MovePayload{Column: botCol})
	h.sendToConn(playerConn, models.MsgGameState, models.GameStatePayload{
		Board:       r.Board,
		CurrentTurn: r.CurrentTurn,
	})

	if winner != 0 || isDraw {
		h.handleGameOver(r, winner, isDraw, winningCells)
	}
}

func (h *Hub) PlayAgain(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client := h.clients[conn]
	if client == nil {
		return
	}

	// Check if previous room was a bot game or room doesn't exist anymore
	r, exists := h.rooms[client.RoomID]

	// For bot games or if room was cleaned up, start a new bot game immediately
	if !exists || (exists && r.IsBotGame) {
		// Clean up old room reference
		if exists {
			delete(h.rooms, client.RoomID)
		}

		// Create new bot game
		roomID := generateRoomID()
		newRoom := room.NewRoom(roomID)
		h.rooms[roomID] = newRoom

		player, _ := newRoom.AddPlayer(conn, client.Username)
		newRoom.AddBotPlayer(bot.BotUsername)

		client.RoomID = roomID
		client.PlayerNum = player.Number

		h.startBotGame(newRoom)
		return
	}

	// For human vs human games
	if client.RoomID == "" {
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

	// If game was active, end it and update metrics
	if r.GameActive {
		h.endGameDueToLeave(r)
	}

	opponent := r.GetOpponent(client.PlayerNum)
	r.RemovePlayer(conn)

	if opponent != nil && opponent.Conn != nil {
		h.sendToConn(opponent.Conn, models.MsgOpponentLeft, nil)
	}

	if r.IsEmpty() {
		delete(h.rooms, client.RoomID)
	}

	client.RoomID = ""
	client.PlayerNum = 0
}

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

	for len(h.waiting) >= 2 {
		p1 := h.waiting[0]
		p2 := h.waiting[1]
		h.waiting = h.waiting[2:]

		roomID := generateRoomID()
		r := room.NewRoom(roomID)
		h.rooms[roomID] = r

		player1, _ := r.AddPlayer(p1.Client.Conn, p1.Client.Username)
		player2, _ := r.AddPlayer(p2.Client.Conn, p2.Client.Username)

		p1.Client.RoomID = roomID
		p1.Client.PlayerNum = player1.Number
		p2.Client.RoomID = roomID
		p2.Client.PlayerNum = player2.Number

		h.startGame(r)
	}
}

func (h *Hub) startGame(r *room.Room) {
	r.StartGame()
	h.notifyGameStart(r)

	if h.kafkaProducer != nil && h.kafkaProducer.IsEnabled() {
		h.kafkaProducer.PublishGameStarted(r.ID, r.Player1.Username, r.Player2.Username, r.IsBotGame)
	} else if h.kafkaConsumer != nil {
		h.kafkaConsumer.IncrementLiveGames()
	}
}

func (h *Hub) notifyGameStart(r *room.Room) {
	p1 := r.Player1
	p2 := r.Player2

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

	if h.kafkaProducer != nil && h.kafkaProducer.IsEnabled() {
		h.kafkaProducer.PublishGameStarted(r.ID, r.Player1.Username, r.Player2.Username, true)
	} else if h.kafkaConsumer != nil {
		h.kafkaConsumer.IncrementLiveGames()
	}
}

func (h *Hub) handleGameOver(r *room.Room, winner int, isDraw bool, winningCells [][2]int) {
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
			WinningCells:  winningCells,
		})
	}

	if p2.Conn != nil {
		h.sendToConn(p2.Conn, models.MsgGameOver, models.GameOverPayload{
			Winner:        winner,
			WinnerName:    winnerName,
			YourScore:     p2.Score,
			OpponentScore: p1.Score,
			WinningCells:  winningCells,
		})
	}

	isBotGame := r.IsBotGame
	roomID := r.ID
	totalMoves := r.MoveCount
	durationSec := r.GetDuration()

	if h.kafkaProducer != nil && h.kafkaProducer.IsEnabled() {
		h.kafkaProducer.PublishGameEnded(roomID, p1.Username, p2.Username, winnerName, isDraw, isBotGame, totalMoves, durationSec)
	} else if h.kafkaConsumer != nil {
		h.kafkaConsumer.DecrementLiveGames()
		h.kafkaConsumer.RecordGameEnd(isBotGame, durationSec, totalMoves)
	}

	go func() {
		err := h.db.SaveGame(database.GameInput{
			RoomID:          roomID,
			Player1Username: p1.Username,
			Player2Username: p2.Username,
			WinnerUsername:  winnerUsername,
			IsDraw:          isDraw,
			IsBotGame:       isBotGame,
			TotalMoves:      totalMoves,
			DurationSeconds: durationSec,
		})
		if err != nil {
			log.Printf("Error saving game: %v", err)
		}
	}()

	if isBotGame {
		go h.cleanupRoom(roomID, p1.Conn)
	}
}

// endGameDueToLeave handles when a player leaves during an active game
func (h *Hub) endGameDueToLeave(r *room.Room) {
	if !r.GameActive {
		return
	}

	r.GameActive = false
	isBotGame := r.IsBotGame
	totalMoves := r.MoveCount
	durationSec := r.GetDuration()

	// Update metrics - decrement live games
	if h.kafkaProducer != nil && h.kafkaProducer.IsEnabled() {
		// Publish game ended event (no winner, player left)
		h.kafkaProducer.PublishGameEnded(r.ID, r.Player1.Username, r.Player2.Username, "", false, isBotGame, totalMoves, durationSec)
	} else if h.kafkaConsumer != nil {
		h.kafkaConsumer.DecrementLiveGames()
		h.kafkaConsumer.RecordGameEnd(isBotGame, durationSec, totalMoves)
	}
}

func (h *Hub) cleanupRoom(roomID string, playerConn *websocket.Conn) {
	time.Sleep(100 * time.Millisecond)

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.rooms, roomID)

	if client, exists := h.clients[playerConn]; exists {
		client.RoomID = ""
		client.PlayerNum = 0
	}
}

func (h *Hub) broadcastOnlineCount() {
	h.mu.RLock()
	count := len(h.clients)
	msg, _ := json.Marshal(models.WSMessage{
		Type:    models.MsgOnlineCount,
		Payload: models.OnlineCountPayload{Count: count},
	})

	sendChans := make([]chan []byte, 0, len(h.clients))
	for _, client := range h.clients {
		sendChans = append(sendChans, client.Send)
	}
	h.mu.RUnlock()

	for _, ch := range sendChans {
		func() {
			defer func() { recover() }()
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
		return
	}

	client := h.clients[conn]
	if client != nil {
		select {
		case client.Send <- msg:
		default:
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
