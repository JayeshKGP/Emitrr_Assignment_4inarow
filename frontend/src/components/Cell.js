import React from 'react';
import './Cell.css';

function Cell({ value, onClick, onMouseEnter, onMouseLeave, isClickable, isHighlighted, isLastMove, currentPlayer }) {
  const getDiscClass = () => {
    if (value === 1) return 'disc red';
    if (value === 2) return 'disc yellow';
    if (isHighlighted) return `disc highlight ${currentPlayer === 1 ? 'highlight-red' : 'highlight-yellow'}`;
    return 'disc empty';
  };

  return (
    <div
      className={`cell ${isClickable ? 'clickable' : ''}`}
      onClick={onClick}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
    >
      <div className={getDiscClass()}>
        {isLastMove && value && <div className="last-move-dot" />}
      </div>
    </div>
  );
}

export default Cell;
