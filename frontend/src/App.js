import React, { useState, useEffect, useCallback } from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import UsernameForm from './components/UsernameForm';
import Lobby from './components/Lobby';
import WaitingRoom from './components/WaitingRoom';
import GameBoard from './components/GameBoard';
import MetricsDashboard from './components/MetricsDashboard';
import wsService from './services/websocket';
import './App.css';

const SCREEN = {
  USERNAME: 'username',
  LOBBY: 'lobby',
  WAITING: 'waiting',
  GAME: 'game',
};

function App() {
  const [screen, setScreen] = useState(SCREEN.USERNAME);
  const [username, setUsername] = useState(() => localStorage.getItem('foursync_username') || '');
  const [playerStats, setPlayerStats] = useState({ totalWins: 0, totalDraws: 0, totalGames: 0, totalScore: 0 });
  const [roomId, setRoomId] = useState('');
  const [gameData, setGameData] = useState(null);
  const [error, setError] = useState('');
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  const setupWebSocketListeners = useCallback(() => {
    wsService.listeners = {};

    wsService.on('login_success', (data) => {
      setUsername(data.username);
      setPlayerStats({
        totalWins: data.totalWins || 0,
        totalDraws: data.totalDraws || 0,
        totalGames: data.totalGames || 0,
        totalScore: data.totalScore || 0,
      });
      localStorage.setItem('foursync_username', data.username);
      setScreen(SCREEN.LOBBY);
      setIsLoggingIn(false);
      setError('');
    });

    wsService.on('room_created', (data) => {
      setRoomId(data.roomId);
      setScreen(SCREEN.WAITING);
    });

    wsService.on('waiting', () => setScreen(SCREEN.WAITING));

    wsService.on('game_start', (data) => {
      setGameData(data);
      setScreen(SCREEN.GAME);
      setError('');
    });

    wsService.on('error', (data) => {
      setError(data?.message || 'An error occurred');
      setIsLoggingIn(false);
    });

    wsService.on('search_timeout', (data) => {
      setError(data?.message || 'No opponent found');
      setScreen(SCREEN.LOBBY);
    });

    wsService.on('bot_assigned', () => {
      setError('No opponent found. Playing against FourSync Bot!');
    });

    wsService.on('opponent_left', () => {
      setError('Opponent has left the game');
      setGameData(null);
      setScreen(SCREEN.LOBBY);
    });

    wsService.on('disconnected', () => {
      setError('Disconnected from server');
      setScreen(SCREEN.LOBBY);
    });
  }, []);

  const handleUsernameSubmit = useCallback(async (name) => {
    setIsLoggingIn(true);
    setError('');
    try {
      await wsService.connect();
      setupWebSocketListeners();
      wsService.login(name);
    } catch (err) {
      setError('Failed to connect to server. Is the backend running?');
      setIsLoggingIn(false);
    }
  }, [setupWebSocketListeners]);

  useEffect(() => {
    const saved = localStorage.getItem('foursync_username');
    if (saved) {
      handleUsernameSubmit(saved);
    }
  }, [handleUsernameSubmit]);

  const handleLogout = () => {
    localStorage.removeItem('foursync_username');
    wsService.disconnect();
    setUsername('');
    setPlayerStats({ totalWins: 0, totalDraws: 0, totalGames: 0, totalScore: 0 });
    setScreen(SCREEN.USERNAME);
    setGameData(null);
    setRoomId('');
  };

  const handleJoinRandom = () => {
    setError('');
    wsService.joinRandom(username);
  };

  const handlePlayBot = () => {
    setError('');
    wsService.playBot();
  };

  const handleJoinRoom = (id) => {
    setError('');
    setRoomId(id);
    wsService.joinRoom(id, username);
  };

  const handleCreateRoom = () => {
    setError('');
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    let newRoomId = '';
    for (let i = 0; i < 6; i++) {
      newRoomId += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    setRoomId(newRoomId);
    wsService.joinRoom(newRoomId, username);
  };

  const handleCancelSearch = () => {
    wsService.cancelSearch();
    setScreen(SCREEN.LOBBY);
    setRoomId('');
  };

  const handleBackToLobby = () => {
    wsService.leaveRoom();
    setScreen(SCREEN.LOBBY);
    setGameData(null);
    setRoomId('');
    setError('');
  };

  const handleOpenMetrics = () => {
    window.open('/analytics', '_blank');
  };

  return (
    <div className="App">
      {screen === SCREEN.USERNAME && (
        <UsernameForm
          onSubmit={handleUsernameSubmit}
          isLoading={isLoggingIn}
          error={error}
        />
      )}

      {screen === SCREEN.LOBBY && (
        <Lobby
          username={username}
          playerStats={playerStats}
          error={error}
          onJoinRandom={handleJoinRandom}
          onPlayBot={handlePlayBot}
          onJoinRoom={handleJoinRoom}
          onCreateRoom={handleCreateRoom}
          onLogout={handleLogout}
          onOpenMetrics={handleOpenMetrics}
        />
      )}

      {screen === SCREEN.WAITING && (
        <WaitingRoom
          roomId={roomId}
          onCancel={handleCancelSearch}
        />
      )}

      {screen === SCREEN.GAME && gameData && (
        <GameBoard
          username={username}
          gameData={gameData}
          onBackToLobby={handleBackToLobby}
        />
      )}
    </div>
  );
}

function AnalyticsPage() {
  return <MetricsDashboard onBack={() => window.close()} />;
}

function AppWithRouter() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<App />} />
        <Route path="/analytics" element={<AnalyticsPage />} />
      </Routes>
    </Router>
  );
}

export default AppWithRouter;
