import React, { useState, useEffect, useCallback } from 'react';
import Cell from './Cell';
import wsService from '../services/websocket';
import './GameBoard.css';

const ROWS = 6;
const COLS = 7;
const MOVE_TIME_LIMIT = 30;
const GAME_TIME_LIMIT = 240;

const createEmptyBoard = () => Array(ROWS).fill(null).map(() => Array(COLS).fill(null));

function GameBoard({ username, gameData, onBackToLobby }) {
  const [board, setBoard] = useState(createEmptyBoard());
  const [currentTurn, setCurrentTurn] = useState(gameData.currentTurn);
  const [hoverColumn, setHoverColumn] = useState(null);
  const [gameStatus, setGameStatus] = useState('playing');
  const [winner, setWinner] = useState(null);
  const [winnerName, setWinnerName] = useState('');
  const [myScore, setMyScore] = useState(gameData.yourScore || 0);
  const [opponentScore, setOpponentScore] = useState(gameData.opponentScore || 0);
  const [playAgainRequested, setPlayAgainRequested] = useState(false);
  const [lastMove, setLastMove] = useState(null); // {row, col} of last move
  const [winningCells, setWinningCells] = useState([]); // cells that caused the win

  const myNumber = gameData.yourNumber;
  const opponent = gameData.opponent;
  const isMyTurn = currentTurn === myNumber;
  const isBotGame = gameData.isBotGame || false;

  // Timer state
  const [gameStartTime, setGameStartTime] = useState(Date.now());
  const [moveStartTime, setMoveStartTime] = useState(Date.now());
  const [gameTimeLeft, setGameTimeLeft] = useState(GAME_TIME_LIMIT);
  const [moveTimeLeft, setMoveTimeLeft] = useState(MOVE_TIME_LIMIT);

  // WebSocket event handlers
  useEffect(() => {
    const handleGameState = (data) => {
      if (data.board) {
        // Convert 0 to null for empty cells
        const newBoard = data.board.map(row =>
          row.map(cell => cell === 0 ? null : cell)
        );

        // Find the last move by comparing with previous board
        setBoard(prevBoard => {
          // Find the new disc position (last move)
          for (let row = 0; row < ROWS; row++) {
            for (let col = 0; col < COLS; col++) {
              const prevCell = prevBoard[row][col];
              const newCell = newBoard[row][col];
              if (prevCell === null && newCell !== null) {
                setLastMove({ row, col });
                break;
              }
            }
          }
          return newBoard;
        });
      }
      setCurrentTurn(data.currentTurn);
      setMoveStartTime(Date.now());
      setMoveTimeLeft(MOVE_TIME_LIMIT);
    };

    const handleOpponentMove = () => {
      // Move received, state will update via game_state
    };

    const handleGameOver = (data) => {
      setGameStatus('finished');
      setWinner(data.winner);
      setWinnerName(data.winnerName);
      setMyScore(data.yourScore);
      setOpponentScore(data.opponentScore);
      if (data.winningCells) {
        setWinningCells(data.winningCells);
      }
    };

    const handleGameStart = (data) => {
      // New game started (play again)
      setBoard(createEmptyBoard());
      setCurrentTurn(data.currentTurn);
      setGameStatus('playing');
      setWinner(null);
      setWinnerName('');
      setMyScore(data.yourScore);
      setOpponentScore(data.opponentScore);
      setGameStartTime(Date.now());
      setMoveStartTime(Date.now());
      setGameTimeLeft(GAME_TIME_LIMIT);
      setMoveTimeLeft(MOVE_TIME_LIMIT);
      setPlayAgainRequested(false);
      setLastMove(null); // Reset last move indicator
      setWinningCells([]); // Reset winning cells
    };

    const handlePlayAgainReq = () => {
      setPlayAgainRequested(true);
    };

    const handleError = (data) => {
      console.error('Game error:', data?.message);
    };

    wsService.on('game_state', handleGameState);
    wsService.on('opponent_move', handleOpponentMove);
    wsService.on('game_over', handleGameOver);
    wsService.on('game_start', handleGameStart);
    wsService.on('play_again_request', handlePlayAgainReq);
    wsService.on('error', handleError);

    return () => {
      wsService.off('game_state', handleGameState);
      wsService.off('opponent_move', handleOpponentMove);
      wsService.off('game_over', handleGameOver);
      wsService.off('game_start', handleGameStart);
      wsService.off('play_again_request', handlePlayAgainReq);
      wsService.off('error', handleError);
    };
  }, []);

  // Find lowest empty row
  const getLowestEmptyRow = useCallback((col) => {
    for (let row = ROWS - 1; row >= 0; row--) {
      if (board[row][col] === null) return row;
    }
    return -1;
  }, [board]);

  // Move timer effect
  useEffect(() => {
    if (gameStatus !== 'playing') return;
    const timer = setInterval(() => {
      const remaining = MOVE_TIME_LIMIT - Math.floor((Date.now() - moveStartTime) / 1000);
      if (remaining <= 0) {
        setMoveTimeLeft(0);
        // Server handles timeout
      } else {
        setMoveTimeLeft(remaining);
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [moveStartTime, gameStatus]);

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

  // Handle column click
  const handleColumnClick = (col) => {
    if (gameStatus !== 'playing' || !isMyTurn) return;
    if (getLowestEmptyRow(col) === -1) return;

    wsService.makeMove(col);
  };

  // Play again
  const handlePlayAgain = () => {
    wsService.playAgain();
    setPlayAgainRequested(true);
  };

  // Leave game
  const handleLeave = () => {
    wsService.leaveRoom();
    onBackToLobby();
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

  const getResultMessage = () => {
    if (winner === 0 || winnerName === 'draw') return "It's a Draw!";
    if (winner === myNumber) return '🎉 You Win!';
    return `${opponent} Wins!`;
  };

  return (
    <div className="game-container">
      {/* Header */}
      <div className="game-header">
        <button className="back-button" onClick={handleLeave}>← Leave</button>
        <h1 className="game-title-small">FourSync</h1>
        <div className="player-info">
          <span className="player-name">{username}</span>
        </div>
      </div>

      {/* Score Display */}
      <div className="score-display">
        <div className={`score-box ${myNumber === 1 ? 'player1' : 'player2'}`}>
          <div className={`player-avatar ${myNumber === 1 ? 'p1' : 'p2'}`}>
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
            </svg>
          </div>
          <span className="score-name">{username}</span>
          <span className="score-value">{myScore}</span>
        </div>
        <span className="score-vs">vs</span>
        <div className={`score-box ${myNumber === 1 ? 'player2' : 'player1'}`}>
          <div className={`player-avatar ${myNumber === 1 ? 'p2' : 'p1'} ${isBotGame ? 'bot' : ''}`}>
            {isBotGame ? (
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2a2 2 0 0 1 2 2c0 .74-.4 1.39-1 1.73V7h1a7 7 0 0 1 7 7h1a1 1 0 0 1 1 1v3a1 1 0 0 1-1 1h-1v1a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-1H2a1 1 0 0 1-1-1v-3a1 1 0 0 1 1-1h1a7 7 0 0 1 7-7h1V5.73c-.6-.34-1-.99-1-1.73a2 2 0 0 1 2-2M7.5 13A1.5 1.5 0 0 0 6 14.5A1.5 1.5 0 0 0 7.5 16A1.5 1.5 0 0 0 9 14.5A1.5 1.5 0 0 0 7.5 13m9 0a1.5 1.5 0 0 0-1.5 1.5a1.5 1.5 0 0 0 1.5 1.5a1.5 1.5 0 0 0 1.5-1.5a1.5 1.5 0 0 0-1.5-1.5M12 9a5 5 0 0 0-5 5v1h10v-1a5 5 0 0 0-5-5z"/>
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
              </svg>
            )}
          </div>
          <span className="score-name">{opponent}</span>
          <span className="score-value">{opponentScore}</span>
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
            <span className={`turn-disc ${currentTurn === 1 ? 'red' : 'yellow'}`} />
            <span className="turn-text">
              {isMyTurn ? 'Your Turn' : `${opponent}'s Turn`}
            </span>
          </>
        )}
        {gameStatus === 'finished' && (
          <span className={winner === myNumber ? 'winner-text' : winner === 0 ? 'draw-text' : 'loser-text'}>
            {getResultMessage()}
          </span>
        )}
        {gameStatus === 'timeout' && <span className="timeout-text">Time's Up!</span>}
      </div>

      {/* Board */}
      <div className="board">
        {board.map((row, rowIndex) => (
          <div key={rowIndex} className="board-row">
            {row.map((cell, colIndex) => {
              const targetRow = getLowestEmptyRow(colIndex);
              const isHighlighted = hoverColumn === colIndex && rowIndex === targetRow && gameStatus === 'playing' && isMyTurn;
              const isLastMove = lastMove && lastMove.row === rowIndex && lastMove.col === colIndex;
              const isWinningCell = winningCells.some(([r, c]) => r === rowIndex && c === colIndex);

              return (
                <Cell
                  key={colIndex}
                  value={cell}
                  onClick={() => handleColumnClick(colIndex)}
                  onMouseEnter={() => setHoverColumn(colIndex)}
                  onMouseLeave={() => setHoverColumn(null)}
                  isClickable={gameStatus === 'playing' && isMyTurn && targetRow !== -1}
                  isHighlighted={isHighlighted}
                  isLastMove={isLastMove}
                  isWinningCell={isWinningCell}
                  currentPlayer={myNumber}
                />
              );
            })}
          </div>
        ))}
      </div>

      {/* Game Over Actions */}
      {gameStatus !== 'playing' && (
        <div className="game-over-actions">
          {playAgainRequested ? (
            <span className="waiting-opponent">Waiting for opponent...</span>
          ) : (
            <button className="reset-button" onClick={handlePlayAgain}>Play Again</button>
          )}
          <button className="menu-button" onClick={handleLeave}>Leave Game</button>
        </div>
      )}
    </div>
  );
}

export default GameBoard;
