import React from 'react';
import './WaitingRoom.css';

function WaitingRoom({ roomId, onCancel }) {
  const copyRoomId = () => {
    navigator.clipboard.writeText(roomId);
  };

  return (
    <div className="waiting-container">
      <div className="waiting-card">
        <div className="waiting-spinner"></div>
        <h2 className="waiting-title">Waiting for opponent...</h2>

        {roomId && (
          <div className="room-info">
            <p className="room-label">Share this room code:</p>
            <div className="room-code-box">
              <span className="room-code">{roomId}</span>
              <button className="copy-btn" onClick={copyRoomId}>Copy</button>
            </div>
          </div>
        )}

        <p className="waiting-hint">
          {roomId
            ? "Share this code with a friend to play together"
            : "Searching for an opponent... (10 second timeout)"}
        </p>

        <button className="cancel-btn" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}

export default WaitingRoom;
