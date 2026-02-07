package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

type Metrics struct {
	mu sync.RWMutex

	TotalGames       int64
	TotalHumanGames  int64
	TotalBotGames    int64
	TotalDurationSec int64
	TotalMoves       int64
	LiveGames        int64
	GamesPerHour     [24]int64
	CurrentHour      int
	LastUpdated      time.Time
}

type CalculatedMetrics struct {
	TotalGames       int64     `json:"total_games"`
	GamesPerHour     float64   `json:"games_per_hour_avg"`
	AvgGameDuration  float64   `json:"avg_game_duration_sec"`
	AvgMovesPerGame  float64   `json:"avg_moves_per_game"`
	LiveGames        int64     `json:"live_games"`
	HumanGames       int64     `json:"human_games"`
	BotGames         int64     `json:"bot_games"`
	GamesLast24Hours [24]int64 `json:"games_last_24_hours"`
}

type Consumer struct {
	consumer sarama.ConsumerGroup
	metrics  *Metrics
	enabled  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

type ConsumerGroupHandler struct {
	metrics *Metrics
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processMessage(msg)
		session.MarkMessage(msg, "")
	}
	return nil
}

func (h *ConsumerGroupHandler) processMessage(msg *sarama.ConsumerMessage) {
	switch msg.Topic {
	case TopicGameEvents:
		var event GameEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return
		}
		h.processGameEvent(event)
	case TopicPlayerEvents:
		var event PlayerEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return
		}
		h.processPlayerEvent(event)
	}
}

func (h *ConsumerGroupHandler) processGameEvent(event GameEvent) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	switch event.EventType {
	case EventGameStarted:
		h.metrics.LiveGames++

	case EventGameEnded:
		if h.metrics.LiveGames > 0 {
			h.metrics.LiveGames--
		}

		h.metrics.TotalGames++

		if event.IsBotGame {
			h.metrics.TotalBotGames++
		} else {
			h.metrics.TotalHumanGames++
		}

		h.metrics.TotalDurationSec += int64(event.DurationSec)
		h.metrics.TotalMoves += int64(event.TotalMoves)

		eventHour := event.Timestamp.Hour()
		h.metrics.GamesPerHour[eventHour]++
		h.metrics.LastUpdated = time.Now()
	}
}

func (h *ConsumerGroupHandler) processPlayerEvent(event PlayerEvent) {
	// Can be extended for player-specific metrics
}

func NewConsumer() *Consumer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:9092"
	}

	if !isKafkaReachable(brokers) {
		return &Consumer{
			enabled: false,
			metrics: &Metrics{CurrentHour: time.Now().Hour()},
		}
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Version = sarama.V2_8_0_0
	config.Net.DialTimeout = 5 * time.Second
	config.Net.ReadTimeout = 30 * time.Second
	config.Net.WriteTimeout = 5 * time.Second
	config.Metadata.Retry.Max = 3
	config.Metadata.Timeout = 5 * time.Second
	config.Consumer.Group.Session.Timeout = 10 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	consumerGroupID := fmt.Sprintf("foursync-metrics-%d", time.Now().UnixNano())

	consumer, err := sarama.NewConsumerGroup([]string{brokers}, consumerGroupID, config)
	if err != nil {
		return &Consumer{
			enabled: false,
			metrics: &Metrics{},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{
		consumer: consumer,
		metrics:  &Metrics{CurrentHour: time.Now().Hour()},
		enabled:  true,
		ctx:      ctx,
		cancel:   cancel,
	}

	go c.run()

	log.Println("Kafka consumer started")
	return c
}

func (c *Consumer) run() {
	handler := &ConsumerGroupHandler{metrics: c.metrics}
	topics := []string{TopicGameEvents, TopicPlayerEvents}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			err := c.consumer.Consume(c.ctx, topics, handler)
			if err != nil && c.ctx.Err() == nil {
				time.Sleep(2 * time.Second)
			}
		}
	}
}

func NewConsumerWithoutKafka() *Consumer {
	return &Consumer{
		enabled: false,
		metrics: &Metrics{CurrentHour: time.Now().Hour()},
	}
}

func (c *Consumer) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.consumer != nil {
		c.consumer.Close()
	}
}

func (c *Consumer) IsEnabled() bool {
	return c.enabled
}

func (c *Consumer) GetMetrics() CalculatedMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	m := c.metrics
	calc := CalculatedMetrics{
		TotalGames:       m.TotalGames,
		LiveGames:        m.LiveGames,
		HumanGames:       m.TotalHumanGames,
		BotGames:         m.TotalBotGames,
		GamesLast24Hours: m.GamesPerHour,
	}

	if m.TotalGames > 0 {
		calc.AvgGameDuration = float64(m.TotalDurationSec) / float64(m.TotalGames)
		calc.AvgMovesPerGame = float64(m.TotalMoves) / float64(m.TotalGames)
	}

	var totalLast24 int64
	for _, count := range m.GamesPerHour {
		totalLast24 += count
	}
	calc.GamesPerHour = float64(totalLast24) / 24.0

	return calc
}

func (c *Consumer) IncrementLiveGames() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics.LiveGames++
}

func (c *Consumer) DecrementLiveGames() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	if c.metrics.LiveGames > 0 {
		c.metrics.LiveGames--
	}
}

func (c *Consumer) RecordGameEnd(isBotGame bool, durationSec, totalMoves int) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.TotalGames++
	c.metrics.TotalDurationSec += int64(durationSec)
	c.metrics.TotalMoves += int64(totalMoves)

	if isBotGame {
		c.metrics.TotalBotGames++
	} else {
		c.metrics.TotalHumanGames++
	}

	currentHour := time.Now().Hour()
	c.metrics.GamesPerHour[currentHour]++
	c.metrics.LastUpdated = time.Now()
}
