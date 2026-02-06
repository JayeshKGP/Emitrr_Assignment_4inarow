package models

import "time"

// Message types for WebSocket communication
const (
	// Client -> Server
	MsgLogin        = "login"
	MsgJoinRoom     = "join_room"
	MsgJoinRandom   = "join_random"
	MsgPlayBot      = "play_bot"
	MsgMakeMove     = "make_move"
	MsgLeaveRoom    = "leave_room"
	MsgPlayAgain    = "play_again"
	MsgCancelSearch = "cancel_search"

	// Server -> Client
	MsgLoginSuccess  = "login_success"
	MsgWaiting       = "waiting"
	MsgBotAssigned   = "bot_assigned"
	MsgGameStart     = "game_start"
	MsgGameState     = "game_state"
	MsgOpponentMove  = "opponent_move"
	MsgGameOver      = "game_over"
	MsgError         = "error"
	MsgRoomCreated   = "room_created"
	MsgOnlineCount   = "online_count"
	MsgOpponentLeft  = "opponent_left"
	MsgPlayAgainReq  = "play_again_request"
	MsgScoreUpdate   = "score_update"
	MsgSearchTimeout = "search_timeout"
)

// WSMessage is the base WebSocket message structure
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// LoginPayload - client login with username
type LoginPayload struct {
	Username string `json:"username"`
}

// LoginSuccessPayload - server sends player info after login
type LoginSuccessPayload struct {
	PlayerID   string `json:"playerId"`
	Username   string `json:"username"`
	TotalWins  int    `json:"totalWins"`
	TotalDraws int    `json:"totalDraws"`
	TotalGames int    `json:"totalGames"`
	TotalScore int    `json:"totalScore"`
}

// JoinRoomPayload - client wants to join a specific room
type JoinRoomPayload struct {
	RoomID   string `json:"roomId"`
	Username string `json:"username"`
}

// JoinRandomPayload - client wants to join random match
type JoinRandomPayload struct {
	Username string `json:"username"`
}

// MovePayload - client makes a move
type MovePayload struct {
	Column int `json:"column"`
}

// RoomCreatedPayload - server sends room ID to creator
type RoomCreatedPayload struct {
	RoomID string `json:"roomId"`
}

// GameStartPayload - game is starting
type GameStartPayload struct {
	RoomID        string `json:"roomId"`
	Opponent      string `json:"opponent"`
	YourNumber    int    `json:"yourNumber"`
	CurrentTurn   int    `json:"currentTurn"`
	YourScore     int    `json:"yourScore"`
	OpponentScore int    `json:"opponentScore"`
	IsBotGame     bool   `json:"isBotGame"`
}

// GameStatePayload - current game state
type GameStatePayload struct {
	Board       [6][7]int `json:"board"`
	CurrentTurn int       `json:"currentTurn"`
}

// GameOverPayload - game ended
type GameOverPayload struct {
	Winner        int    `json:"winner"` // 0=draw, 1 or 2=winner
	WinnerName    string `json:"winnerName"`
	YourScore     int    `json:"yourScore"`
	OpponentScore int    `json:"opponentScore"`
}

// ErrorPayload - error message
type ErrorPayload struct {
	Message string `json:"message"`
}

// OnlineCountPayload - number of online players
type OnlineCountPayload struct {
	Count int `json:"count"`
}

// ScoreUpdatePayload - score update
type ScoreUpdatePayload struct {
	YourScore     int `json:"yourScore"`
	OpponentScore int `json:"opponentScore"`
}

// Player represents a connected player
type Player struct {
	ID        string
	Username  string
	Conn      interface{} // WebSocket connection
	RoomID    string
	Score     int
	CreatedAt time.Time
}

// Game represents a game record for database
type Game struct {
	ID          string    `json:"id"`
	RoomID      string    `json:"roomId"`
	Player1     string    `json:"player1"`
	Player2     string    `json:"player2"`
	Winner      string    `json:"winner"` // username or "draw"
	TotalMoves  int       `json:"totalMoves"`
	Duration    int       `json:"duration"` // seconds
	CreatedAt   time.Time `json:"createdAt"`
	CompletedAt time.Time `json:"completedAt"`
}
