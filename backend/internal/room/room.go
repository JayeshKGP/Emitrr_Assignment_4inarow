package room

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/game"
)

// Player in a room
type Player struct {
	Conn     *websocket.Conn
	Username string
	Number   int // 1 or 2
	Score    int
}

// Room represents a game room
type Room struct {
	ID          string
	Player1     *Player
	Player2     *Player
	Board       [6][7]int
	CurrentTurn int
	GameEngine  *game.Engine
	StartTime   time.Time
	MoveCount   int
	GameActive  bool
	IsBotGame   bool
	mu          sync.Mutex
}

// NewRoom creates a new room
func NewRoom(id string) *Room {
	return &Room{
		ID:          id,
		CurrentTurn: 1,
		GameEngine:  game.NewEngine(),
		GameActive:  false,
	}
}

// AddPlayer adds a player to the room
func (r *Room) AddPlayer(conn *websocket.Conn, username string) (*Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Player1 == nil {
		r.Player1 = &Player{Conn: conn, Username: username, Number: 1, Score: 0}
		return r.Player1, nil
	}
	if r.Player2 == nil {
		r.Player2 = &Player{Conn: conn, Username: username, Number: 2, Score: 0}
		return r.Player2, nil
	}
	return nil, ErrRoomFull
}

// AddBotPlayer adds bot as player 2
func (r *Room) AddBotPlayer(botUsername string) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Player2 = &Player{Conn: nil, Username: botUsername, Number: 2, Score: 0}
	r.IsBotGame = true
	return r.Player2
}

// GetBoard returns copy of current board
func (r *Room) GetBoard() [6][7]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Board
}

// GetCurrentTurn returns current turn
func (r *Room) GetCurrentTurn() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.CurrentTurn
}

// IsGameActive returns if game is active
func (r *Room) IsGameActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.GameActive
}

// RemovePlayer removes a player from the room
func (r *Room) RemovePlayer(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Player1 != nil && r.Player1.Conn == conn {
		r.Player1 = nil
	}
	if r.Player2 != nil && r.Player2.Conn == conn {
		r.Player2 = nil
	}
}

// IsFull checks if room has 2 players
func (r *Room) IsFull() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Player1 != nil && r.Player2 != nil
}

// IsEmpty checks if room has no players
func (r *Room) IsEmpty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Player1 == nil && r.Player2 == nil
}

// StartGame initializes a new game
func (r *Room) StartGame() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Board = [6][7]int{}
	r.CurrentTurn = 1
	r.StartTime = time.Now()
	r.MoveCount = 0
	r.GameActive = true
}

// MakeMove processes a move
func (r *Room) MakeMove(playerNum, col int) (row int, winner int, isDraw bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.GameActive {
		return -1, 0, false, ErrGameNotActive
	}

	if r.CurrentTurn != playerNum {
		return -1, 0, false, ErrNotYourTurn
	}

	if !r.GameEngine.IsValidMove(r.Board, col) {
		return -1, 0, false, ErrInvalidMove
	}

	row = r.GameEngine.MakeMove(&r.Board, col, playerNum)
	r.MoveCount++

	if r.GameEngine.CheckWin(r.Board, row, col, playerNum) {
		r.GameActive = false
		return row, playerNum, false, nil
	}

	if r.GameEngine.IsBoardFull(r.Board) {
		r.GameActive = false
		return row, 0, true, nil
	}

	// Switch turn
	r.CurrentTurn = 3 - playerNum
	return row, 0, false, nil
}

// GetOpponent returns the opponent player
func (r *Room) GetOpponent(playerNum int) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerNum == 1 {
		return r.Player2
	}
	return r.Player1
}

// GetPlayer returns the player by number
func (r *Room) GetPlayer(playerNum int) *Player {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerNum == 1 {
		return r.Player1
	}
	return r.Player2
}

// UpdateScores updates scores based on winner
func (r *Room) UpdateScores(winner int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if winner == 0 { // Draw
		r.Player1.Score++
		r.Player2.Score++
	} else if winner == 1 {
		r.Player1.Score += 2
	} else {
		r.Player2.Score += 2
	}
}

// GetDuration returns game duration in seconds
func (r *Room) GetDuration() int {
	return int(time.Since(r.StartTime).Seconds())
}

// Errors
var (
	ErrRoomFull      = &RoomError{"room is full"}
	ErrGameNotActive = &RoomError{"game is not active"}
	ErrNotYourTurn   = &RoomError{"not your turn"}
	ErrInvalidMove   = &RoomError{"invalid move"}
)

type RoomError struct {
	msg string
}

func (e *RoomError) Error() string {
	return e.msg
}
