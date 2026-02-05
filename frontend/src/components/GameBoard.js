import React, { useState, useEffect, useCallback } from 'react';
import Cell from './Cell';
import './GameBoard.css';

// Game constants
const ROWS = 6;
const COLS = 7;
const MOVE_TIME_LIMIT = 30;
const GAME_TIME_LIMIT = 240; // 4 minutes

const createEmptyBoard = () => Array(ROWS).fill(null).map(() => Array(COLS).fill(null));

function GameBoard({ username, onBackToMenu }) {
  // Game state
  const [board, setBoard] = useState(createEmptyBoard());
  const [currentPlayer, setCurrentPlayer] = useState(1);
  const [hoverColumn, setHoverColumn] = useState(null);
  const [gameStatus, setGameStatus] = useState('playing');
  const [winner, setWinner] = useState(null);

  // Timer state
  const [gameStartTime, setGameStartTime] = useState(Date.now());
  const [moveStartTime, setMoveStartTime] = useState(Date.now());
  const [gameTimeLeft, setGameTimeLeft] = useState(GAME_TIME_LIMIT);
  const [moveTimeLeft, setMoveTimeLeft] = useState(MOVE_TIME_LIMIT);

  // Find lowest empty row in a column (-1 if full)
  const getLowestEmptyRow = (col) => {
    for (let row = ROWS - 1; row >= 0; row--) {
      if (board[row][col] === null) return row;
    }
    return -1;
  };

  // Switch turn on move timeout
  const handleMoveTimeout = useCallback(() => {
    if (gameStatus !== 'playing') return;
    setCurrentPlayer(prev => prev === 1 ? 2 : 1);
    setMoveStartTime(Date.now());
    setMoveTimeLeft(MOVE_TIME_LIMIT);
  }, [gameStatus]);

  // Move timer effect
  useEffect(() => {
    if (gameStatus !== 'playing') return;
    const timer = setInterval(() => {
      const remaining = MOVE_TIME_LIMIT - Math.floor((Date.now() - moveStartTime) / 1000);
      if (remaining <= 0) handleMoveTimeout();
      else setMoveTimeLeft(remaining);
    }, 1000);
    return () => clearInterval(timer);
  }, [moveStartTime, gameStatus, handleMoveTimeout]);

  // Game timer effect
  useEffect(() => {
    if (gameStatus !== 'playing') return;
    const timer = setInterval(() => {
      const remaining = GAME_TIME_LIMIT - Math.floor((Date.now() - gameStartTime) / 1000);
      if (remaining <= 0) {
        setGameStatus('timeout');
        setGameTimeLeft(0);
      } else {
        setGameTimeLeft(remaining);
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [gameStartTime, gameStatus]);

  // Check for 4 in a row from last move position
  const checkWin = (board, row, col, player) => {
    const directions = [[0, 1], [1, 0], [1, 1], [1, -1]];

    for (const [dRow, dCol] of directions) {
      let count = 1;

      // Count in positive direction
      for (let r = row + dRow, c = col + dCol;
           r >= 0 && r < ROWS && c >= 0 && c < COLS && board[r][c] === player;
           r += dRow, c += dCol) count++;

      // Count in negative direction
      for (let r = row - dRow, c = col - dCol;
           r >= 0 && r < ROWS && c >= 0 && c < COLS && board[r][c] === player;
           r -= dRow, c -= dCol) count++;

      if (count >= 4) return true;
    }
    return false;
  };

  // Check if board is full (draw)
  const checkDraw = (board) => board[0].every(cell => cell !== null);

  // Handle player move
  const handleColumnClick = (col) => {
    if (gameStatus !== 'playing') return;

    const row = getLowestEmptyRow(col);
    if (row === -1) return;

    const newBoard = board.map(r => [...r]);
    newBoard[row][col] = currentPlayer;
    setBoard(newBoard);

    if (checkWin(newBoard, row, col, currentPlayer)) {
      setGameStatus('won');
      setWinner(currentPlayer);
      return;
    }

    if (checkDraw(newBoard)) {
      setGameStatus('draw');
      return;
    }

    setCurrentPlayer(currentPlayer === 1 ? 2 : 1);
    setMoveStartTime(Date.now());
    setMoveTimeLeft(MOVE_TIME_LIMIT);
  };

  // Reset game to initial state
  const handleResetGame = () => {
    setBoard(createEmptyBoard());
    setCurrentPlayer(1);
    setGameStatus('playing');
    setWinner(null);
    setGameStartTime(Date.now());
    setMoveStartTime(Date.now());
    setGameTimeLeft(GAME_TIME_LIMIT);
    setMoveTimeLeft(MOVE_TIME_LIMIT);
  };

  const formatTime = (seconds) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const getTimerClass = (value, warnAt, criticalAt) => {
    if (value <= criticalAt) return 'timer-box critical';
    if (value <= warnAt) return 'timer-box warning';
    return 'timer-box';
  };

  return (
    <div className="game-container">
      {/* Header */}
      <div className="game-header">
        <button className="back-button" onClick={onBackToMenu}>← Back</button>
        <h1 className="game-title-small">FourSync</h1>
        <div className="player-info">
          <span className="player-name">{username}</span>
        </div>
      </div>

      {/* Timers */}
      <div className="timers-container">
        <div className={getTimerClass(gameTimeLeft, 60, 30)}>
          <span className="timer-label">Game</span>
          <span className="timer-value">{formatTime(gameTimeLeft)}</span>
        </div>
        <div className={getTimerClass(moveTimeLeft, 10, 5)}>
          <span className="timer-label">Move</span>
          <span className="timer-value">{moveTimeLeft}s</span>
        </div>
      </div>

      {/* Turn Indicator */}
      <div className="turn-indicator">
        {gameStatus === 'playing' && (
          <>
            <span className={`turn-disc ${currentPlayer === 1 ? 'red' : 'yellow'}`} />
            <span className="turn-text">
              {currentPlayer === 1 ? `${username}'s Turn` : "Opponent's Turn"}
            </span>
          </>
        )}
        {gameStatus === 'won' && (
          <span className="winner-text">
            🎉 {winner === 1 ? `${username} Wins!` : 'Opponent Wins!'}
          </span>
        )}
        {gameStatus === 'draw' && <span className="draw-text">It's a Draw!</span>}
        {gameStatus === 'timeout' && <span className="timeout-text">Time's Up!</span>}
      </div>

      {/* Board */}
      <div className="board">
        {board.map((row, rowIndex) => (
          <div key={rowIndex} className="board-row">
            {row.map((cell, colIndex) => {
              const targetRow = getLowestEmptyRow(colIndex);
              const isHighlighted = hoverColumn === colIndex && rowIndex === targetRow && gameStatus === 'playing';

              return (
                <Cell
                  key={colIndex}
                  value={cell}
                  onClick={() => handleColumnClick(colIndex)}
                  onMouseEnter={() => setHoverColumn(colIndex)}
                  onMouseLeave={() => setHoverColumn(null)}
                  isClickable={gameStatus === 'playing' && targetRow !== -1}
                  isHighlighted={isHighlighted}
                  currentPlayer={currentPlayer}
                />
              );
            })}
          </div>
        ))}
      </div>

      {/* Game Over Actions */}
      {gameStatus !== 'playing' && (
        <div className="game-over-actions">
          <button className="reset-button" onClick={handleResetGame}>Play Again</button>
          <button className="menu-button" onClick={onBackToMenu}>Main Menu</button>
        </div>
      )}
    </div>
  );
}

export default GameBoard;
