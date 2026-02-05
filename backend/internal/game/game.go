package game

// Board constants
const (
	Rows    = 6
	Cols    = 7
	Empty   = 0
	Player1 = 1
	Player2 = 2
)

// Engine handles game logic
type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

// MakeMove places a disc in the column, returns row (-1 if invalid)
func (e *Engine) MakeMove(board *[6][7]int, col, player int) int {
	if col < 0 || col >= Cols {
		return -1
	}
	for row := Rows - 1; row >= 0; row-- {
		if board[row][col] == Empty {
			board[row][col] = player
			return row
		}
	}
	return -1
}

// CheckWin checks if the last move resulted in a win
func (e *Engine) CheckWin(board [6][7]int, row, col, player int) bool {
	directions := [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

	for _, dir := range directions {
		count := 1
		// Positive direction
		for r, c := row+dir[0], col+dir[1]; r >= 0 && r < Rows && c >= 0 && c < Cols && board[r][c] == player; r, c = r+dir[0], c+dir[1] {
			count++
		}
		// Negative direction
		for r, c := row-dir[0], col-dir[1]; r >= 0 && r < Rows && c >= 0 && c < Cols && board[r][c] == player; r, c = r-dir[0], c-dir[1] {
			count++
		}
		if count >= 4 {
			return true
		}
	}
	return false
}

// IsBoardFull checks for draw
func (e *Engine) IsBoardFull(board [6][7]int) bool {
	for col := 0; col < Cols; col++ {
		if board[0][col] == Empty {
			return false
		}
	}
	return true
}

// IsValidMove checks if move is valid
func (e *Engine) IsValidMove(board [6][7]int, col int) bool {
	return col >= 0 && col < Cols && board[0][col] == Empty
}
