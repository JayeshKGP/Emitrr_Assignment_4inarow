import React, { useState } from 'react';
import UsernameForm from './components/UsernameForm';
import GameBoard from './components/GameBoard';
import './App.css';

function App() {
  const [username, setUsername] = useState(null);

  return (
    <div className="App">
      {!username ? (
        <UsernameForm onSubmit={setUsername} />
      ) : (
        <GameBoard username={username} onBackToMenu={() => setUsername(null)} />
      )}
    </div>
  );
}

export default App;
