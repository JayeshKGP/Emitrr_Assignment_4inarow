package bot

import (
	"math"
)

const (
	BotUsername = "FourSync Bot"
	Rows        = 6
	Cols        = 7
	MaxDepth    = 6
)

const (
	Empty    = 0
	Player1  = 1
	Player2  = 2
	BotPlayer = Player2
)

func GetBestMove(board [6][7]int, botPlayerNum int) int {
	_, move := minimax(board, MaxDepth, math.MinInt32, math.MaxInt32, true, botPlayerNum)
	if move == -1 {
		for col := 0; col < Cols; col++ {
			if board[0][col] == Empty {
				return col
			}
		}
	}
	return move
}

func minimax(board [6][7]int, depth int, alpha, beta int, isMaximizing bool, botPlayerNum int) (int, int) {
	opponentNum := Player1
	if botPlayerNum == Player1 {
		opponentNum = Player2
	}

	if winner := checkWinner(board); winner != 0 {
		if winner == botPlayerNum {
			return 1000000 + depth, -1
		}
		return -1000000 - depth, -1
	}

	if isBoardFull(board) {
		return 0, -1
	}

	if depth == 0 {
		return evaluateBoard(board, botPlayerNum), -1
	}

	validMoves := getValidMoves(board)
	if len(validMoves) == 0 {
		return 0, -1
	}

	orderedMoves := orderMoves(validMoves)
	bestMove := orderedMoves[0]

	if isMaximizing {
		maxEval := math.MinInt32
		for _, col := range orderedMoves {
			newBoard := makeMove(board, col, botPlayerNum)
			eval, _ := minimax(newBoard, depth-1, alpha, beta, false, botPlayerNum)
			if eval > maxEval {
				maxEval = eval
				bestMove = col
			}
			alpha = max(alpha, eval)
			if beta <= alpha {
				break
			}
		}
		return maxEval, bestMove
	} else {
		minEval := math.MaxInt32
		for _, col := range orderedMoves {
			newBoard := makeMove(board, col, opponentNum)
			eval, _ := minimax(newBoard, depth-1, alpha, beta, true, botPlayerNum)
			if eval < minEval {
				minEval = eval
				bestMove = col
			}
			beta = min(beta, eval)
			if beta <= alpha {
				break
			}
		}
		return minEval, bestMove
	}
}

func evaluateBoard(board [6][7]int, botPlayerNum int) int {
	score := 0
	opponentNum := Player1
	if botPlayerNum == Player1 {
		opponentNum = Player2
	}

	centerCol := Cols / 2
	centerCount := 0
	for row := 0; row < Rows; row++ {
		if board[row][centerCol] == botPlayerNum {
			centerCount++
		}
	}
	score += centerCount * 6

	score += evaluateLines(board, botPlayerNum, opponentNum)

	return score
}

func evaluateLines(board [6][7]int, botPlayerNum, opponentNum int) int {
	score := 0

	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols-3; col++ {
			score += evaluateWindow(board[row][col], board[row][col+1], board[row][col+2], board[row][col+3], botPlayerNum, opponentNum)
		}
	}

	for col := 0; col < Cols; col++ {
		for row := 0; row < Rows-3; row++ {
			score += evaluateWindow(board[row][col], board[row+1][col], board[row+2][col], board[row+3][col], botPlayerNum, opponentNum)
		}
	}

	for row := 0; row < Rows-3; row++ {
		for col := 0; col < Cols-3; col++ {
			score += evaluateWindow(board[row][col], board[row+1][col+1], board[row+2][col+2], board[row+3][col+3], botPlayerNum, opponentNum)
		}
	}

	for row := 3; row < Rows; row++ {
		for col := 0; col < Cols-3; col++ {
			score += evaluateWindow(board[row][col], board[row-1][col+1], board[row-2][col+2], board[row-3][col+3], botPlayerNum, opponentNum)
		}
	}

	return score
}

func evaluateWindow(a, b, c, d, botPlayerNum, opponentNum int) int {
	botCount := 0
	oppCount := 0
	emptyCount := 0

	for _, cell := range []int{a, b, c, d} {
		if cell == botPlayerNum {
			botCount++
		} else if cell == opponentNum {
			oppCount++
		} else {
			emptyCount++
		}
	}

	if botCount == 4 {
		return 10000
	}
	if oppCount == 4 {
		return -10000
	}

	if botCount == 3 && emptyCount == 1 {
		return 100
	}
	if oppCount == 3 && emptyCount == 1 {
		return -80
	}

	if botCount == 2 && emptyCount == 2 {
		return 10
	}
	if oppCount == 2 && emptyCount == 2 {
		return -8
	}

	return 0
}

func checkWinner(board [6][7]int) int {
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols-3; col++ {
			if board[row][col] != Empty &&
				board[row][col] == board[row][col+1] &&
				board[row][col] == board[row][col+2] &&
				board[row][col] == board[row][col+3] {
				return board[row][col]
			}
		}
	}

	for col := 0; col < Cols; col++ {
		for row := 0; row < Rows-3; row++ {
			if board[row][col] != Empty &&
				board[row][col] == board[row+1][col] &&
				board[row][col] == board[row+2][col] &&
				board[row][col] == board[row+3][col] {
				return board[row][col]
			}
		}
	}

	for row := 0; row < Rows-3; row++ {
		for col := 0; col < Cols-3; col++ {
			if board[row][col] != Empty &&
				board[row][col] == board[row+1][col+1] &&
				board[row][col] == board[row+2][col+2] &&
				board[row][col] == board[row+3][col+3] {
				return board[row][col]
			}
		}
	}

	for row := 3; row < Rows; row++ {
		for col := 0; col < Cols-3; col++ {
			if board[row][col] != Empty &&
				board[row][col] == board[row-1][col+1] &&
				board[row][col] == board[row-2][col+2] &&
				board[row][col] == board[row-3][col+3] {
				return board[row][col]
			}
		}
	}

	return 0
}

func isBoardFull(board [6][7]int) bool {
	for col := 0; col < Cols; col++ {
		if board[0][col] == Empty {
			return false
		}
	}
	return true
}

func getValidMoves(board [6][7]int) []int {
	moves := []int{}
	for col := 0; col < Cols; col++ {
		if board[0][col] == Empty {
			moves = append(moves, col)
		}
	}
	return moves
}

func orderMoves(moves []int) []int {
	center := Cols / 2
	ordered := make([]int, len(moves))
	copy(ordered, moves)

	for i := 0; i < len(ordered)-1; i++ {
		for j := i + 1; j < len(ordered); j++ {
			distI := abs(ordered[i] - center)
			distJ := abs(ordered[j] - center)
			if distJ < distI {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}
	return ordered
}

func makeMove(board [6][7]int, col, player int) [6][7]int {
	newBoard := board
	for row := Rows - 1; row >= 0; row-- {
		if newBoard[row][col] == Empty {
			newBoard[row][col] = player
			break
		}
	}
	return newBoard
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
