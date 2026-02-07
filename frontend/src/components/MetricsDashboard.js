import React, { useState, useEffect, useCallback } from 'react';
import './MetricsDashboard.css';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080';

function MetricsDashboard({ onBack }) {
  const [metrics, setMetrics] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchMetrics = useCallback(async () => {
    try {
      const response = await fetch(`${API_URL}/api/metrics`);
      if (!response.ok) throw new Error('Failed to fetch metrics');
      const data = await response.json();
      setMetrics(data);
      setLoading(false);
    } catch (err) {
      setError(err.message);
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchMetrics();
    const interval = setInterval(fetchMetrics, 5000);
    return () => clearInterval(interval);
  }, [fetchMetrics]);

  if (loading) {
    return (
      <div className="metrics-container">
        <div className="metrics-card">
          <div className="loading">Loading metrics...</div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="metrics-container">
        <div className="metrics-card">
          <div className="error">Error: {error}</div>
          <button className="close-btn" onClick={onBack}>Close</button>
        </div>
      </div>
    );
  }

  return (
    <div className="metrics-container">
      <div className="metrics-card">
        <div className="metrics-header">
          <h1>Game Analytics</h1>
          <div className="header-actions">
            <span className="live-badge">Live</span>
            <button className="close-btn" onClick={onBack}>Close</button>
          </div>
        </div>

        <div className="metrics-section">
          <h2>Real-Time</h2>
          <div className="metrics-grid">
            <div className="metric-box live">
              <span className="metric-value">{metrics.live_games ?? 0}</span>
              <span className="metric-label">Live Games</span>
            </div>
            <div className="metric-box total">
              <span className="metric-value">{metrics.total_games ?? 0}</span>
              <span className="metric-label">Total Games</span>
            </div>
          </div>
        </div>

        <div className="metrics-section">
          <h2>Game Types</h2>
          <div className="metrics-grid">
            <div className="metric-box human">
              <span className="metric-value">{metrics.human_games ?? 0}</span>
              <span className="metric-label">Human vs Human</span>
            </div>
            <div className="metric-box bot">
              <span className="metric-value">{metrics.bot_games ?? 0}</span>
              <span className="metric-label">Human vs Bot</span>
            </div>
          </div>
        </div>

        <div className="metrics-section">
          <h2>Performance</h2>
          <div className="metrics-grid three-col">
            <div className="metric-box">
              <span className="metric-value">
                {metrics.avg_game_duration_sec?.toFixed(0) ?? '0'}s
              </span>
              <span className="metric-label">Avg Duration</span>
            </div>
            <div className="metric-box">
              <span className="metric-value">
                {metrics.avg_moves_per_game?.toFixed(0) ?? '0'}
              </span>
              <span className="metric-label">Avg Moves</span>
            </div>
            <div className="metric-box">
              <span className="metric-value">
                {metrics.games_per_hour_avg?.toFixed(1) ?? '0'}
              </span>
              <span className="metric-label">Games/Hour</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default MetricsDashboard;
