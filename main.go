package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	lip "github.com/charmbracelet/lipgloss"
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

		// initial player
		currentPlayer: "X",

		// our to-do list is a grocery list
		board: [][]string{{"", "", ""}, {"", "", ""}, {"", "", ""}},
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
			m.board = [][]string{{"", "", ""}, {"", "", ""}, {"", "", ""}}
			m.currentPlayer = "X"
			m.winner = ""
			m.winningCells = nil
			m.cursorX, m.cursorY = 0, 0

		// the "enter" and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			// ignore moves if the game is already over
			if m.winner != "" {
				break
			}

			// only place on empty cells
			if m.board[m.cursorY][m.cursorX] != "" {
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
				m.winner = "draw"
			} else {
				// swap turns
				if m.currentPlayer == "X" {
					m.currentPlayer = "O"
				} else {
					m.currentPlayer = "X"
				}
			}
		}
	}

	// return the updated model to the Bubble Tea runtime for processing.
	// Note: we're not returning a command
	return m, nil
}

func styledPlayer(player string) string {
	if player == "X" {
		return xStyle.Render("X")
	}
	return oStyle.Render("O")
}

func (m model) View() string {
	// header
	s := "\n"
	s += headerStyle.Render(`
  _____ _       _____           _____         
 |_   _(_)__ __|_   _|_ _ __ __|_   _|___  ___ 
   | | | / _'___|| |/ _' / _'___|| \|/ _ \/ -_)
   |_| |_\__|    |_|\__,_\__|    |_| \___/\___|
`)
	s += "\n\n"

	// cursor style
	cursorStyle := lip.NewStyle().Background(lip.Color("#44475a")).Foreground(lip.Color("#f8f8f2")).Bold(true) // dark background

	for y, row := range m.board {
		s += "\t\t"
		for x, cell := range row {

			// build cell content
			var content string
			switch cell {
			case "X":
				content = "X"
			case "O":
				content = "O"
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
				fullCell = winStyle.Render(fullCell)
			} else if m.cursorX == x && m.cursorY == y {
				// cursor takes priority over normal colors
				var styled lip.Style
				switch cell {
				case "X":
					styled = cursorStyle.Foreground(lip.Color("#8BE9FD"))
				case "O":
					styled = cursorStyle.Foreground(lip.Color("#FF79C6"))
				default:
					styled = cursorStyle
				}

				fullCell = styled.Render(fullCell)
			} else {
				switch cell {
				case "X":
					fullCell = xStyle.Render(fullCell)
				case "O":
					fullCell = oStyle.Render(fullCell)
				default:
					fullCell = cellStyle.Render(fullCell)
				}
			}

			s += fullCell
		}
		s += "\n"
	}

	// footer
	if m.winner == "draw" {
		s += footerStyle.Render("\nIt's a draw! Press r to restart, q to quit.\n")
	} else if m.winner != "" {
		s += footerStyle.Render("\nWinner: ") + styledPlayer(m.winner) + "\n"
	} else {
		s += footerStyle.Render("\nCurrent turn: ") + styledPlayer(m.currentPlayer) + "\n"
	}

	s += footerStyle.Render("\nPress r to restart, q to quit\n")

	return s
}

// TODO: Use something like this as the winning screen.
// Maybe there's a way to redraw a 'separate' view upon win condition?
/*
░██    ░██    ░██       ░██ ░██
 ░██  ░██     ░██       ░██
  ░██░██      ░██  ░██  ░██ ░██░████████   ░███████
   ░███       ░██ ░████ ░██ ░██░██    ░██ ░██
  ░██░██      ░██░██ ░██░██ ░██░██    ░██  ░███████
 ░██  ░██     ░████   ░████ ░██░██    ░██        ░██
░██    ░██    ░███     ░███ ░██░██    ░██  ░███████
*/

/*
  ░██████      ░██       ░██ ░██
 ░██   ░██     ░██       ░██
░██     ░██    ░██  ░██  ░██ ░██░████████   ░███████
░██     ░██    ░██ ░████ ░██ ░██░██    ░██ ░██
░██     ░██    ░██░██ ░██░██ ░██░██    ░██  ░███████
 ░██   ░██     ░████   ░████ ░██░██    ░██        ░██
  ░██████      ░███     ░███ ░██░██    ░██  ░█████
*/

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
