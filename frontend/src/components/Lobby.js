import React, { useState } from 'react';
import './Lobby.css';

function Lobby({ username, playerStats, error, onJoinRandom, onPlayBot, onJoinRoom, onCreateRoom, onLogout, onOpenMetrics }) {
  const [roomInput, setRoomInput] = useState('');
  const [showJoinRoom, setShowJoinRoom] = useState(false);

  const handleJoinRoom = (e) => {
    e.preventDefault();
    if (roomInput.trim().length >= 4) {
      onJoinRoom(roomInput.trim().toUpperCase());
    }
  };

  return (
    <div className="lobby-container">
      <div className="lobby-card">
        <div className="lobby-header">
          <h1 className="lobby-title">FourSync</h1>
          <div className="user-info">
            <span className="username-display">{username}</span>
            <button className="logout-btn" onClick={onLogout}>Logout</button>
          </div>
        </div>

        <div className="player-stats">
          <div className="stat-item">
            <span className="stat-value">{playerStats.totalGames ?? 0}</span>
            <span className="stat-label">Games</span>
          </div>
          <div className="stat-item wins">
            <span className="stat-value">{playerStats.totalWins ?? 0}</span>
            <span className="stat-label">Wins</span>
          </div>
          <div className="stat-item draws">
            <span className="stat-value">{playerStats.totalDraws ?? 0}</span>
            <span className="stat-label">Draws</span>
          </div>
          <div className="stat-item score">
            <span className="stat-value">{playerStats.totalScore ?? 0}</span>
            <span className="stat-label">Score</span>
          </div>
        </div>

        <button className="analytics-btn" onClick={onOpenMetrics}>
          <span className="analytics-icon">📊</span>
          <span>View Analytics</span>
        </button>

        {error && <div className="error-banner">{error}</div>}

        <div className="lobby-actions">
          <button className="action-btn primary" onClick={onJoinRandom}>
            <span className="btn-icon">🎲</span>
            <span className="btn-text">
              <strong>Random Match</strong>
              <small>Find an opponent automatically</small>
            </span>
          </button>

          <button className="action-btn bot" onClick={onPlayBot}>
            <span className="btn-icon">🤖</span>
            <span className="btn-text">
              <strong>Play vs Bot</strong>
              <small>Challenge the FourSync Bot</small>
            </span>
          </button>

          <button className="action-btn secondary" onClick={onCreateRoom}>
            <span className="btn-icon">➕</span>
            <span className="btn-text">
              <strong>Create Room</strong>
              <small>Get a code to share with a friend</small>
            </span>
          </button>

          <button
            className="action-btn secondary"
            onClick={() => setShowJoinRoom(!showJoinRoom)}
          >
            <span className="btn-icon">🔗</span>
            <span className="btn-text">
              <strong>Join Room</strong>
              <small>Enter a room code</small>
            </span>
          </button>

          {showJoinRoom && (
            <form className="join-room-form" onSubmit={handleJoinRoom}>
              <input
                type="text"
                value={roomInput}
                onChange={(e) => setRoomInput(e.target.value.toUpperCase())}
                placeholder="Enter Room Code"
                maxLength={6}
                className="room-input"
                autoFocus
              />
              <button type="submit" className="join-btn" disabled={roomInput.length < 4}>
                Join
              </button>
            </form>
          )}
        </div>

        <div className="lobby-footer">
          <p>Connect 4 discs in a row to win!</p>
        </div>
      </div>
    </div>
  );
}

export default Lobby;
