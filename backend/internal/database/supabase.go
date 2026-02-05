package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client for Supabase REST API
type Client struct {
	URL    string
	APIKey string
	client *http.Client
}

// GameRecord represents a game to save (uses player IDs for 3NF)
type GameRecord struct {
	RoomID          string  `json:"room_id"`
	Player1ID       string  `json:"player1_id"`
	Player2ID       string  `json:"player2_id"`
	WinnerID        *string `json:"winner_id"`
	IsDraw          bool    `json:"is_draw"`
	TotalMoves      int     `json:"total_moves"`
	DurationSeconds int     `json:"duration_seconds"`
}

// GameInput for SaveGame method (uses usernames)
type GameInput struct {
	RoomID          string
	Player1Username string
	Player2Username string
	WinnerUsername  *string
	IsDraw          bool
	TotalMoves      int
	DurationSeconds int
}

// LeaderboardEntry from database view
type LeaderboardEntry struct {
	Username   string  `json:"username"`
	TotalWins  int     `json:"total_wins"`
	TotalGames int     `json:"total_games"`
	TotalScore int     `json:"total_score"`
	WinRate    float64 `json:"win_rate"`
	Rank       int     `json:"rank"`
}

// PlayerResponse from get_or_create_player RPC
type PlayerResponse struct {
	ID string `json:"id"`
}

// PlayerWithStats contains player info and stats for login
type PlayerWithStats struct {
	PlayerID   string `json:"player_id"`
	Username   string `json:"username"`
	TotalWins  int    `json:"total_wins"`
	TotalGames int    `json:"total_games"`
	TotalScore int    `json:"total_score"`
}

// NewClient creates a new Supabase client
func NewClient() *Client {
	return &Client{
		URL:    os.Getenv("SUPABASE_URL"),
		APIKey: os.Getenv("SUPABASE_ANON_KEY"),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetOrCreatePlayer gets existing player or creates new one, returns player ID
func (c *Client) GetOrCreatePlayer(username string) (string, error) {
	if c.URL == "" || c.APIKey == "" {
		return "", fmt.Errorf("supabase not configured")
	}

	// Call the RPC function
	payload := map[string]string{"p_username": username}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.URL+"/rest/v1/rpc/get_or_create_player", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("supabase error: %s", string(body))
	}

	// Parse UUID response (returned as plain string with quotes)
	var playerID string
	if err := json.NewDecoder(resp.Body).Decode(&playerID); err != nil {
		return "", err
	}

	return playerID, nil
}

// LoginPlayer gets or creates player and returns full stats
func (c *Client) LoginPlayer(username string) (*PlayerWithStats, error) {
	if c.URL == "" || c.APIKey == "" {
		return nil, fmt.Errorf("supabase not configured")
	}

	// First get or create the player
	playerID, err := c.GetOrCreatePlayer(username)
	if err != nil {
		return nil, err
	}

	// Now fetch their stats from the leaderboard view
	url := fmt.Sprintf("%s/rest/v1/leaderboard?username=eq.%s", c.URL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []LeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	// Return player with stats (new player will have 0 stats)
	stats := &PlayerWithStats{
		PlayerID:   playerID,
		Username:   username,
		TotalWins:  0,
		TotalGames: 0,
		TotalScore: 0,
	}

	if len(entries) > 0 {
		stats.TotalWins = entries[0].TotalWins
		stats.TotalGames = entries[0].TotalGames
		stats.TotalScore = entries[0].TotalScore
	}

	return stats, nil
}

// SaveGame saves a completed game to database
func (c *Client) SaveGame(input GameInput) error {
	if c.URL == "" || c.APIKey == "" {
		return fmt.Errorf("supabase not configured")
	}

	// Get or create player IDs
	player1ID, err := c.GetOrCreatePlayer(input.Player1Username)
	if err != nil {
		return fmt.Errorf("failed to get player1: %w", err)
	}

	player2ID, err := c.GetOrCreatePlayer(input.Player2Username)
	if err != nil {
		return fmt.Errorf("failed to get player2: %w", err)
	}

	// Determine winner ID
	var winnerID *string
	if input.WinnerUsername != nil {
		if *input.WinnerUsername == input.Player1Username {
			winnerID = &player1ID
		} else {
			winnerID = &player2ID
		}
	}

	// Create game record with player IDs
	game := GameRecord{
		RoomID:          input.RoomID,
		Player1ID:       player1ID,
		Player2ID:       player2ID,
		WinnerID:        winnerID,
		IsDraw:          input.IsDraw,
		TotalMoves:      input.TotalMoves,
		DurationSeconds: input.DurationSeconds,
	}

	data, err := json.Marshal(game)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.URL+"/rest/v1/games", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Prefer", "return=minimal")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase error: %s", string(body))
	}

	return nil
}

// GetLeaderboard fetches top players from the leaderboard view
func (c *Client) GetLeaderboard(limit int) ([]LeaderboardEntry, error) {
	if c.URL == "" || c.APIKey == "" {
		return nil, fmt.Errorf("supabase not configured")
	}

	url := fmt.Sprintf("%s/rest/v1/leaderboard?select=*&limit=%d", c.URL, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supabase error: %s", string(body))
	}

	var entries []LeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// GetPlayerStats fetches stats for a specific player
func (c *Client) GetPlayerStats(username string) (*LeaderboardEntry, error) {
	if c.URL == "" || c.APIKey == "" {
		return nil, fmt.Errorf("supabase not configured")
	}

	url := fmt.Sprintf("%s/rest/v1/leaderboard?username=eq.%s", c.URL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", c.APIKey)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []LeaderboardEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return nil, nil
	}

	return &entries[0], nil
}
