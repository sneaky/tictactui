package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	lip "github.com/charmbracelet/lipgloss"
)

// Game constants
const (
	PlayerX = "X"
	PlayerO = "O"
	Draw    = "draw"
	Empty   = ""
)

// style colors
var (
	winStyle    = lip.NewStyle().Foreground(lip.Color("#50FA7B")).Bold(true) // dracula green green & bold
	xStyle      = lip.NewStyle().Foreground(lip.Color("#8BE9FD"))            // dracula cyan
	oStyle      = lip.NewStyle().Foreground(lip.Color("#FF79C6"))            // dracula pink
	headerStyle = lip.NewStyle().Foreground(lip.Color("#F1FA8C")).Bold(true) // dracula yellow
	footerStyle = lip.NewStyle().Foreground(lip.Color("#6272A4")).Bold(true) // dracula comment blue
	cellStyle   = lip.NewStyle().Foreground(lip.Color("#BD93F9"))            // dracula purple
)

type coord struct {
	row int
	col int
}

type model struct {
	board            [][]string // game board
	cursorX, cursorY int        // which cell our cursor is currently on
	currentPlayer    string     //"X" or "O"
	winner           string     // "", "X", or "O"
	winningCells     []coord    // allows us to highlight winning cells at win
}

func initialModel() model {
	return model{
		currentPlayer: PlayerX,
		board:         [][]string{{Empty, Empty, Empty}, {Empty, Empty, Empty}, {Empty, Empty, Empty}},
	}
}

// resetGame resets the game to initial state
func (m *model) resetGame() {
	m.board = [][]string{{Empty, Empty, Empty}, {Empty, Empty, Empty}, {Empty, Empty, Empty}}
	m.currentPlayer = PlayerX
	m.winner = Empty
	m.winningCells = nil
	m.cursorX, m.cursorY = 0, 0
}

// switchPlayer toggles between X and O
func (m *model) switchPlayer() {
	if m.currentPlayer == PlayerX {
		m.currentPlayer = PlayerO
	} else {
		m.currentPlayer = PlayerX
	}
}

func (m model) Init() tea.Cmd {
	// just return 'nil', which means "no I/O right now, please."
	return nil
}

func checkWinner(board [][]string, player string) []coord {
	// rows
	for y := 0; y < 3; y++ {
		if board[y][0] == player && board[y][1] == player && board[y][2] == player {
			return []coord{{y, 0}, {y, 1}, {y, 2}}
		}
	}

	// cols
	for x := 0; x < 3; x++ {
		if board[0][x] == player && board[1][x] == player && board[2][x] == player {
			return []coord{{0, x}, {1, x}, {2, x}}
		}
	}

	// diagonals
	if board[0][0] == player && board[1][1] == player && board[2][2] == player {
		return []coord{{0, 0}, {1, 1}, {2, 2}}
	}
	if board[0][2] == player && board[1][1] == player && board[2][0] == player {
		return []coord{{0, 2}, {1, 1}, {2, 0}}
	}

	// no current winner
	return nil
}

// check for draw
func isDraw(board [][]string) bool {
	for _, row := range board {
		for _, cell := range row {
			if cell == "" {
				return false
			}
		}
	}
	return true
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// is it a key press?
	case tea.KeyMsg:

		// cool, what key was pressed?
		switch msg.String() {

		// these keys should exit the program
		case "ctrl+c", "q":
			return m, tea.Quit

		// the "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursorY > 0 {
				m.cursorY--
			}

		// the "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursorY < len(m.board)-1 {
				m.cursorY++
				if m.cursorX >= len(m.board[m.cursorY]) {
					m.cursorX = len(m.board[m.cursorY]) - 1
				}
			}

		// the "right" and "l" keys move the cursor right
		case "right", "l":
			if m.cursorX < len(m.board[m.cursorY])-1 {
				m.cursorX++
			}

		// the "left" and "h" keys move the cursor right
		case "left", "h":
			if m.cursorX > 0 {
				m.cursorX--
			}

		// reset the game
		case "r":
			m.resetGame()
			return m, tea.ClearScreen

		// the "enter" and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			// ignore moves if the game is already over
			if m.winner != Empty {
				break
			}

			// only place on empty cells
			if m.board[m.cursorY][m.cursorX] != Empty {
				// TODO: make this flash to alert user
				break
			}

			// place the move
			m.board[m.cursorY][m.cursorX] = m.currentPlayer

			cells := checkWinner(m.board, m.currentPlayer)
			if cells != nil {
				m.winner = m.currentPlayer
				m.winningCells = cells
			} else if isDraw(m.board) {
				m.winner = Draw
			} else {
				m.switchPlayer()
			}
		}
	}

	// return the updated model to the Bubble Tea runtime for processing.
	// Note: we're not returning a command
	return m, nil
}

func styledPlayer(player string) string {
	if player == PlayerX {
		return xStyle.Render(PlayerX)
	}
	return oStyle.Render(PlayerO)
}

// renderCell creates a styled cell for the game board
func (m model) renderCell(x, y int, cell string) string {
	// build cell content
	var content string
	switch cell {
	case PlayerX:
		content = PlayerX
	case PlayerO:
		content = PlayerO
	default:
		content = " "
	}
	fullCell := "[" + content + "]"

	// check if this cell is part of a winning combo
	highlight := false
	for _, c := range m.winningCells {
		if c.row == y && c.col == x {
			highlight = true
			break
		}
	}

	// apply styles
	if highlight {
		return winStyle.Render(fullCell)
	} else if m.cursorX == x && m.cursorY == y {
		// cursor takes priority over normal colors
		cursorStyle := lip.NewStyle().Background(lip.Color("#44475a")).Foreground(lip.Color("#f8f8f2")).Bold(true)
		var styled lip.Style
		switch cell {
		case PlayerX:
			styled = cursorStyle.Foreground(lip.Color("#8BE9FD"))
		case PlayerO:
			styled = cursorStyle.Foreground(lip.Color("#FF79C6"))
		default:
			styled = cursorStyle
		}
		return styled.Render(fullCell)
	} else {
		switch cell {
		case PlayerX:
			return xStyle.Render(fullCell)
		case PlayerO:
			return oStyle.Render(fullCell)
		default:
			return cellStyle.Render(fullCell)
		}
	}
}

func (m model) View() string {
	// If there's a winner, show full screen ASCII art
	switch m.winner {
	case PlayerX:
		return showXWinScreen()
	case PlayerO:
		return showOWinScreen()
	case Draw:
		return showDrawScreen()
	}

	// Normal game view
	// header
	s := "\n"
	s += headerStyle.Render(`
  _____ _       _____           _____         
 |_   _(_)__ __|_   _|_ _ __ __|_   _|___  ___ 
   | | | / _'___|| |/ _' / _'___|| | / _ \/ -_)
   |_| |_\__|    |_|\__,_\__|    |_| \___/\___|
`)
	s += "\n\n"

	for y, row := range m.board {
		s += "\t\t"
		for x, cell := range row {
			s += m.renderCell(x, y, cell)
		}
		s += "\n"
	}

	// footer
	s += footerStyle.Render("\nCurrent turn: ") + styledPlayer(m.currentPlayer) + "\n"
	s += footerStyle.Render("\nPress r to restart, q to quit\n")

	return s
}

func showXWinScreen() string {
	s := "\n\n\n"
	s += xStyle.Render(`
░██    ░██    ░██       ░██ ░██
 ░██  ░██     ░██       ░██
  ░██░██      ░██  ░██  ░██ ░██░████████   ░███████
   ░███       ░██ ░████ ░██ ░██░██    ░██ ░██
  ░██░██      ░██░██ ░██░██ ░██░██    ░██  ░███████
 ░██  ░██     ░████   ░████ ░██░██    ░██        ░██
░██    ░██    ░███     ░███ ░██░██    ░██  ░███████
`)
	s += "\n\n"
	s += footerStyle.Render("Press r to restart, q to quit\n")
	return s
}

func showOWinScreen() string {
	s := "\n\n\n"
	s += oStyle.Render(`
  ░██████      ░██       ░██ ░██
 ░██   ░██     ░██       ░██
░██     ░██    ░██  ░██  ░██ ░██░████████   ░███████
░██     ░██    ░██ ░████ ░██ ░██░██    ░██ ░██
░██     ░██    ░██░██ ░██░██ ░██░██    ░██  ░███████
 ░██   ░██     ░████   ░████ ░██░██    ░██        ░██
  ░██████      ░███     ░███ ░██░██    ░██  ░█████
`)
	s += "\n\n"
	s += footerStyle.Render("Press r to restart, q to quit\n")
	return s
}

func showDrawScreen() string {
	s := "\n\n\n"
	s += headerStyle.Render(`
░███████                                         
░██   ░██                                        
░██    ░██ ░██░████  ░██████   ░██    ░██    ░██ 
░██    ░██ ░███           ░██  ░██    ░██    ░██ 
░██    ░██ ░██       ░███████   ░██  ░████  ░██  
░██   ░██  ░██      ░██   ░██    ░██░██ ░██░██   
░███████   ░██       ░█████░██    ░███   ░███    
`)
	s += "\n\n"
	s += footerStyle.Render("It's a draw! Press r to restart, q to quit\n")
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
