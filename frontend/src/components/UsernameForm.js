import React, { useState } from 'react';
import './UsernameForm.css';

function UsernameForm({ onSubmit }) {
  const [username, setUsername] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    const trimmed = username.trim();

    if (trimmed.length < 3) {
      setError('Username must be at least 3 characters');
      return;
    }
    if (trimmed.length > 15) {
      setError('Username must be less than 15 characters');
      return;
    }

    onSubmit(trimmed);
  };

  return (
    <div className="username-container">
      <div className="username-card">
        <h1 className="game-title">FourSync</h1>
        <p className="game-subtitle">Connect Four Discs to Win!</p>

        <form onSubmit={handleSubmit} className="username-form">
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Enter your username"
            className="username-input"
            autoFocus
          />
          {error && <p className="error-message">{error}</p>}
          <button type="submit" className="play-button">Play Game</button>
        </form>

        <div className="game-rules">
          <h3>How to Play</h3>
          <ul>
            <li>Players take turns dropping discs into columns</li>
            <li>Discs fall to the lowest available space</li>
            <li>Connect 4 discs horizontally, vertically, or diagonally to win</li>
          </ul>
        </div>
      </div>
    </div>
  );
}

export default UsernameForm;
