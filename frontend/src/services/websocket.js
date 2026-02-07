// WebSocket service for game communication

const WS_URL = process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws';

class WebSocketService {
  constructor() {
    this.ws = null;
    this.listeners = {};
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
  }

  connect() {
    return new Promise((resolve, reject) => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        resolve();
        return;
      }

      this.ws = new WebSocket(WS_URL);

      const connectionTimeout = setTimeout(() => {
        reject(new Error('Connection timeout'));
      }, 10000);

      this.ws.onopen = () => {
        clearTimeout(connectionTimeout);
        this.reconnectAttempts = 0;
        resolve();
      };

      this.ws.onclose = () => {
        clearTimeout(connectionTimeout);
        this.emit('disconnected', null);
        this.attemptReconnect();
      };

      this.ws.onerror = (error) => {
        clearTimeout(connectionTimeout);
        reject(error);
      };

      this.ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          this.emit(message.type, message.payload);
        } catch (err) {
          // Silent fail for parse errors
        }
      };
    });
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.emit('reconnect_failed', null);
      return;
    }

    this.reconnectAttempts++;
    setTimeout(() => {
      this.connect().catch(() => {});
    }, 2000 * this.reconnectAttempts);
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  send(type, payload = {}) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type, payload }));
    }
  }

  on(event, callback) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(callback);
  }

  off(event, callback) {
    if (this.listeners[event]) {
      this.listeners[event] = this.listeners[event].filter(cb => cb !== callback);
    }
  }

  emit(event, data) {
    if (this.listeners[event]) {
      this.listeners[event].forEach(callback => callback(data));
    }
  }

  // Game actions
  login(username) {
    this.send('login', { username });
  }

  joinRoom(roomId, username) {
    this.send('join_room', { roomId, username });
  }

  joinRandom(username) {
    this.send('join_random', { username });
  }

  playBot() {
    this.send('play_bot', {});
  }

  makeMove(column) {
    this.send('make_move', { column });
  }

  leaveRoom() {
    this.send('leave_room');
  }

  playAgain() {
    this.send('play_again');
  }

  cancelSearch() {
    this.send('cancel_search');
  }
}

const wsService = new WebSocketService();
export default wsService;
