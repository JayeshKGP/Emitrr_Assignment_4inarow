package kafka

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"github.com/IBM/sarama"
)

func isKafkaReachable(broker string) bool {
	conn, err := net.DialTimeout("tcp", broker, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

const (
	TopicGameEvents   = "foursync.game.events"
	TopicPlayerEvents = "foursync.player.events"
)

const (
	EventGameStarted  = "GAME_STARTED"
	EventGameEnded    = "GAME_ENDED"
	EventPlayerLogin  = "PLAYER_LOGIN"
	EventPlayerLogout = "PLAYER_LOGOUT"
)

type GameEvent struct {
	EventType   string    `json:"event_type"`
	Timestamp   time.Time `json:"timestamp"`
	RoomID      string    `json:"room_id"`
	Player1     string    `json:"player1"`
	Player2     string    `json:"player2"`
	IsBotGame   bool      `json:"is_bot_game"`
	Winner      string    `json:"winner,omitempty"`
	IsDraw      bool      `json:"is_draw,omitempty"`
	TotalMoves  int       `json:"total_moves,omitempty"`
	DurationSec int       `json:"duration_sec,omitempty"`
}

type PlayerEvent struct {
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Username  string    `json:"username"`
	PlayerID  string    `json:"player_id,omitempty"`
}

type Producer struct {
	producer sarama.SyncProducer
	enabled  bool
}

func NewProducer() *Producer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	if !isKafkaReachable(brokers) {
		return &Producer{enabled: false}
	}

	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 1
	config.Producer.Return.Successes = true
	config.Net.DialTimeout = 3 * time.Second
	config.Net.ReadTimeout = 3 * time.Second
	config.Net.WriteTimeout = 3 * time.Second
	config.Metadata.Retry.Max = 1
	config.Metadata.Timeout = 3 * time.Second

	producer, err := sarama.NewSyncProducer([]string{brokers}, config)
	if err != nil {
		return &Producer{enabled: false}
	}

	log.Println("Kafka producer connected")
	return &Producer{
		producer: producer,
		enabled:  true,
	}
}

func (p *Producer) Close() {
	if p.producer != nil {
		p.producer.Close()
	}
}

func (p *Producer) IsEnabled() bool {
	return p.enabled
}

func (p *Producer) PublishGameStarted(roomID, player1, player2 string, isBotGame bool) {
	if !p.enabled {
		return
	}

	event := GameEvent{
		EventType: EventGameStarted,
		Timestamp: time.Now(),
		RoomID:    roomID,
		Player1:   player1,
		Player2:   player2,
		IsBotGame: isBotGame,
	}

	p.publishGameEvent(event)
}

func (p *Producer) PublishGameEnded(roomID, player1, player2, winner string, isDraw, isBotGame bool, totalMoves, durationSec int) {
	if !p.enabled {
		return
	}

	event := GameEvent{
		EventType:   EventGameEnded,
		Timestamp:   time.Now(),
		RoomID:      roomID,
		Player1:     player1,
		Player2:     player2,
		Winner:      winner,
		IsDraw:      isDraw,
		IsBotGame:   isBotGame,
		TotalMoves:  totalMoves,
		DurationSec: durationSec,
	}

	p.publishGameEvent(event)
}

func (p *Producer) PublishPlayerLogin(username, playerID string) {
	if !p.enabled {
		return
	}

	event := PlayerEvent{
		EventType: EventPlayerLogin,
		Timestamp: time.Now(),
		Username:  username,
		PlayerID:  playerID,
	}

	p.publishPlayerEvent(event)
}

func (p *Producer) PublishPlayerLogout(username string) {
	if !p.enabled {
		return
	}

	event := PlayerEvent{
		EventType: EventPlayerLogout,
		Timestamp: time.Now(),
		Username:  username,
	}

	p.publishPlayerEvent(event)
}

func (p *Producer) publishGameEvent(event GameEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicGameEvents,
		Key:   sarama.StringEncoder(event.RoomID),
		Value: sarama.ByteEncoder(data),
	}

	p.producer.SendMessage(msg)
}

func (p *Producer) publishPlayerEvent(event PlayerEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	msg := &sarama.ProducerMessage{
		Topic: TopicPlayerEvents,
		Key:   sarama.StringEncoder(event.Username),
		Value: sarama.ByteEncoder(data),
	}

	p.producer.SendMessage(msg)
}
