package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/database"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/hub"
	"github.com/jayeshdeshmukh/connect-four-backend/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Handler holds dependencies
type Handler struct {
	Hub *hub.Hub
}

// NewHandler creates a new handler
func NewHandler(h *hub.Hub) *Handler {
	return &Handler{Hub: h}
}

// HandleWebSocket handles WebSocket connections
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := h.Hub.RegisterClient(conn)

	// Start goroutines for reading and writing
	go h.writePump(conn, client)
	go h.readPump(conn)
}

// readPump reads messages from the WebSocket
func (h *Handler) readPump(conn *websocket.Conn) {
	defer func() {
		h.Hub.UnregisterClient(conn)
		conn.Close()
	}()

	conn.SetReadLimit(4096)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		h.handleMessage(conn, message)
	}
}

// writePump writes messages to the WebSocket
func (h *Handler) writePump(conn *websocket.Conn, client *hub.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages
func (h *Handler) handleMessage(conn *websocket.Conn, message []byte) {
	var msg models.WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error parsing message: %v", err)
		return
	}

	switch msg.Type {
	case models.MsgLogin:
		var payload models.LoginPayload
		if err := parsePayload(msg.Payload, &payload); err != nil {
			log.Printf("Error parsing login payload: %v", err)
			return
		}
		h.Hub.Login(conn, payload.Username)

	case models.MsgJoinRoom:
		var payload models.JoinRoomPayload
		if err := parsePayload(msg.Payload, &payload); err != nil {
			log.Printf("Error parsing join_room payload: %v", err)
			return
		}
		h.Hub.JoinRoom(conn, payload.RoomID, payload.Username)

	case models.MsgJoinRandom:
		var payload models.JoinRandomPayload
		if err := parsePayload(msg.Payload, &payload); err != nil {
			log.Printf("Error parsing join_random payload: %v", err)
			return
		}
		h.Hub.JoinRandom(conn, payload.Username)

	case models.MsgMakeMove:
		var payload models.MovePayload
		if err := parsePayload(msg.Payload, &payload); err != nil {
			log.Printf("Error parsing make_move payload: %v", err)
			return
		}
		h.Hub.MakeMove(conn, payload.Column)

	case models.MsgLeaveRoom:
		h.Hub.LeaveRoom(conn)

	case models.MsgPlayAgain:
		h.Hub.PlayAgain(conn)

	case models.MsgCancelSearch:
		h.Hub.CancelSearch(conn)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

func parsePayload(payload interface{}, target interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// HealthCheck endpoint
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "ok",
		"onlinePlayers": h.Hub.GetOnlineCount(),
	})
}

// GetLeaderboard returns top players
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	db := database.NewClient()
	leaderboard, err := db.GetLeaderboard(20)
	if err != nil {
		log.Printf("Error fetching leaderboard: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to fetch leaderboard"})
		return
	}

	json.NewEncoder(w).Encode(leaderboard)
}
