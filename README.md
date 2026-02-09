# FourSync - Multiplayer Connect Four Game

A real-time multiplayer Connect Four game built with React and Go, featuring WebSocket communication, bot opponent with Minimax AI, and live analytics powered by Kafka.

## Live Demo

- **Frontend:** https://foursync.jdcodes.space/
- **Backend API:** https://api.foursync.jdcodes.space/

## Features

- **Real-time Multiplayer** - Play against other players with WebSocket-based communication
- **Random Matchmaking** - Auto-match with available players or get assigned a bot after 10 seconds
- **Private Rooms** - Create/join rooms with unique codes to play with friends
- **AI Bot Opponent** - Play against a bot powered by Minimax algorithm with alpha-beta pruning
- **Live Game Timers** - 30-second move timer and 4-minute game timer
- **Leaderboard** - Track player rankings with wins, draws, and scores
- **Analytics Dashboard** - Real-time metrics including live games, total games, and average game duration
- **Persistent Stats** - Player statistics stored in Supabase (PostgreSQL)

## Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | React.js |
| Backend | Go (Golang) |
| Database | Supabase (PostgreSQL) |
| Real-time | WebSocket (Gorilla) |
| Analytics | Apache Kafka |
| Hosting | AWS EC2 (Backend), S3 + CloudFront (Frontend) |

## Local Setup

### Prerequisites

- Node.js 18+
- Go 1.21+
- Docker & Docker Compose (for Kafka)

### Kafka (Optional - for analytics)

```bash
docker-compose -f docker-compose.kafka.yml up -d
```

### Backend

```bash
cd backend

# Create .env file with required variables (see below)

# Start server
go run main.go
```

Server runs at `http://localhost:8080`

### Frontend

```bash
cd frontend

# Create .env file with required variables (see below)

# Install dependencies
npm install

# Start development server
npm start
```

App runs at `http://localhost:3000`

## Environment Variables

### Backend (.env)

```
PORT=8080
ALLOWED_ORIGINS=http://localhost:3000
SUPABASE_URL=your_supabase_url
SUPABASE_ANON_KEY=your_supabase_anon_key
KAFKA_BROKERS=localhost:9092
```

### Frontend (.env)

```
REACT_APP_API_URL=http://localhost:8080
REACT_APP_WS_URL=ws://localhost:8080/ws
```

## Database Schema

The app uses Supabase with the following structure:

- `players` - Player profiles
- `games` - Game history
- `player_stats` - Player statistics
- `leaderboard` - View for player rankings with win rate

## Deployment

- **Frontend**: Built with `npm run build`, deployed to S3, served via CloudFront CDN
- **Backend**: Deployed on EC2, managed with PM2 process manager
- **Kafka**: Running on EC2 via Docker Compose
