import React, { useState, useEffect } from 'react';
import './Leaderboard.css';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

function Leaderboard({ onBack }) {
  const [leaderboard, setLeaderboard] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchLeaderboard();
  }, []);

  const fetchLeaderboard = async () => {
    try {
      setLoading(true);
      const response = await fetch(`${API_URL}/api/leaderboard`);
      if (!response.ok) {
        throw new Error('Failed to fetch leaderboard');
      }
      const data = await response.json();
      setLeaderboard(data || []);
      setError('');
    } catch (err) {
      setError('Failed to load leaderboard');
      console.error('Leaderboard error:', err);
    } finally {
      setLoading(false);
    }
  };

  const getRankDisplay = (rank) => {
    switch (rank) {
      case 1: return { icon: '🥇', class: 'gold' };
      case 2: return { icon: '🥈', class: 'silver' };
      case 3: return { icon: '🥉', class: 'bronze' };
      default: return { icon: null, class: '' };
    }
  };

  return (
    <div className="leaderboard-container">
      <div className="leaderboard-content">
        {/* Header */}
        <div className="leaderboard-header">
          <button className="back-btn" onClick={onBack}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M19 12H5M12 19l-7-7 7-7"/>
            </svg>
            Back
          </button>
          <div className="header-center">
            <h1 className="leaderboard-title">
              <span className="trophy-icon">🏆</span>
              Leaderboard
            </h1>
            <p className="leaderboard-subtitle">Top FourSync Players</p>
          </div>
          <button className="refresh-btn" onClick={fetchLeaderboard} disabled={loading}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className={loading ? 'spinning' : ''}>
              <path d="M23 4v6h-6M1 20v-6h6"/>
              <path d="M3.51 9a9 9 0 0114.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0020.49 15"/>
            </svg>
          </button>
        </div>

        {/* Loading State */}
        {loading && (
          <div className="loading-state">
            <div className="loader">
              <div className="loader-ring"></div>
              <div className="loader-ring"></div>
              <div className="loader-ring"></div>
            </div>
            <p>Loading leaderboard...</p>
          </div>
        )}

        {/* Error State */}
        {error && !loading && (
          <div className="error-state">
            <span className="error-icon">⚠️</span>
            <p>{error}</p>
            <button onClick={fetchLeaderboard}>Try Again</button>
          </div>
        )}

        {/* Empty State */}
        {!loading && !error && leaderboard.length === 0 && (
          <div className="empty-state">
            <div className="empty-illustration">
              <span>🎮</span>
            </div>
            <h3>No Players Yet!</h3>
            <p>Be the first to play and claim the top spot!</p>
          </div>
        )}

        {/* Leaderboard Content */}
        {!loading && !error && leaderboard.length > 0 && (
          <>
            {/* All Players Table */}
            <div className="players-list">
              <div className="list-header">
                <span className="col-rank">#</span>
                <span className="col-player">Player</span>
                <span className="col-record">Record</span>
                <span className="col-winrate">Win Rate</span>
                <span className="col-score">Score</span>
              </div>
              <div className="list-body">
                {leaderboard.map((player, index) => {
                  const rank = index + 1;
                  const rankInfo = getRankDisplay(rank);
                  return (
                    <div key={player.username} className={`player-row ${rankInfo.class}`}>
                      <span className="col-rank">
                        {rankInfo.icon ? (
                          <span className="rank-medal">{rankInfo.icon}</span>
                        ) : (
                          <span className="rank-badge">{rank}</span>
                        )}
                      </span>
                      <span className="col-player">
                        <div className={`player-avatar-small ${rankInfo.class}`}>
                          {player.username.charAt(0).toUpperCase()}
                        </div>
                        <span className="player-name">{player.username}</span>
                      </span>
                      <span className="col-record">
                        <span className="record-wins">{player.total_wins}</span>
                        <span className="record-sep">-</span>
                        <span className="record-draws">{player.total_draws}</span>
                        <span className="record-sep">-</span>
                        <span className="record-losses">{player.total_games - player.total_wins - player.total_draws}</span>
                      </span>
                      <span className="col-winrate">
                        <div className="winrate-bar">
                          <div
                            className="winrate-fill"
                            style={{ width: `${Math.min(player.win_rate || 0, 100)}%` }}
                          ></div>
                        </div>
                        <span className="winrate-text">{(player.win_rate || 0).toFixed(0)}%</span>
                      </span>
                      <span className="col-score">
                        <span className="score-value">{player.total_score}</span>
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Stats Legend */}
            <div className="legend">
              <div className="legend-item">
                <span className="legend-color wins"></span>
                <span>Wins (+2 pts)</span>
              </div>
              <div className="legend-item">
                <span className="legend-color draws"></span>
                <span>Draws (+1 pt)</span>
              </div>
              <div className="legend-item">
                <span className="legend-color losses"></span>
                <span>Losses (0 pts)</span>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export default Leaderboard;
