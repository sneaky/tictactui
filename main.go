package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	lip "github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
)

/*
   TODO:
       1: Create a matchmaking service that keeps track of:
           a: Who is looking for a match
           b: Assigns players to each other
           c: Records results (wins/losses)
       2: Extract that matchmaking service into its own package so it can be used for creating/hosting other multiplayer TUI apps
       3: Future Ideas:
           a: Tournament Mode: An even number of players >=4 join a Tournament Mode Server and are randomly paired up then compete tournament style
           b: More board games! (checkers, chess, solitaire, connect-4 (this one would be VERY cool if we could figure out the dropping mechanic))
           c: Add a web leaderboard so people can show off
*/

// Game constants
const (
	PlayerX = "X"
	PlayerO = "O"
	Draw    = "draw"
	Empty   = ""

	// Board dimensions
	BoardSize = 3

	// Ticker frequency for real-time updates (100ms)
	TickerInterval = time.Millisecond * 100

	// Disconnect timeout
	DisconnectTimeout = 5 * time.Second
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

// Global session manager
var (
	sessionManager = &SessionManager{
		waitingSession: nil,
		mutex:          sync.RWMutex{},
	}
)

type SessionManager struct {
	waitingSession *GameSession
	mutex          sync.RWMutex
}

type GameSession struct {
	Board              [][]string
	CurrentPlayer      int
	Winner             string
	WinningCells       []coord
	PlayerCount        int
	PlayerDisconnected bool
	RestartRequested   bool
	mutex              sync.RWMutex
}

type tickMsg time.Time

type model struct {
	board            [][]string   // game board
	cursorX, cursorY int          // which cell our cursor is currently on
	currentPlayer    string       //"X" or "O"
	winner           string       // "", "X", or "O"
	winningCells     []coord      // allows us to highlight winning cells at win
	playerSymbol     string       // "X" or "O" - which player this is
	isMyTurn         bool         // whether it's this player's turn
	waitingForPlayer bool         // whether waiting for another player
	gameSession      *GameSession // shared game session
	disconnectTimer  time.Time    // when disconnect was detected
}

// createEmptyBoard creates a new empty 3x3 board
func createEmptyBoard() [][]string {
	board := make([][]string, BoardSize)
	for i := range board {
		board[i] = make([]string, BoardSize)
		for j := range board[i] {
			board[i][j] = Empty
		}
	}
	return board
}

// copyBoard creates a deep copy of the board
func copyBoard(board [][]string) [][]string {
	newBoard := make([][]string, len(board))
	for i, row := range board {
		newBoard[i] = make([]string, len(row))
		copy(newBoard[i], row)
	}
	return newBoard
}

func initialModel() model {
	return model{
		currentPlayer: PlayerX,
		board:         createEmptyBoard(),
	}
}

// resetGame resets the game to initial state
func (m *model) resetGame() {
	m.board = createEmptyBoard()
	m.currentPlayer = PlayerX
	m.winner = Empty
	m.winningCells = nil
	m.cursorX, m.cursorY = 0, 0
	m.isMyTurn = m.playerSymbol == m.currentPlayer
	m.disconnectTimer = time.Time{} // Reset disconnect timer

	// Reset shared session if in multiplayer mode
	if m.gameSession != nil {
		m.gameSession.mutex.Lock()
		m.gameSession.Board = createEmptyBoard()
		m.gameSession.CurrentPlayer = 0
		m.gameSession.Winner = Empty
		m.gameSession.WinningCells = nil
		m.gameSession.PlayerDisconnected = false // Reset disconnect status
		m.gameSession.RestartRequested = false   // Reset restart status
		m.gameSession.mutex.Unlock()
	}
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
	// Start ticker for real-time updates if in multiplayer mode
	if m.gameSession != nil {
		return tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return nil
}

func checkWinner(board [][]string, player string) []coord {
	// rows
	for y := 0; y < BoardSize; y++ {
		if board[y][0] == player && board[y][1] == player && board[y][2] == player {
			return []coord{{y, 0}, {y, 1}, {y, 2}}
		}
	}

	// cols
	for x := 0; x < BoardSize; x++ {
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

	// Handle tick messages for real-time updates
	case tickMsg:
		if m.gameSession != nil {
			// Sync with session state
			m.gameSession.mutex.RLock()
			m.board = copyBoard(m.gameSession.Board)
			if m.gameSession.CurrentPlayer == 0 {
				m.currentPlayer = PlayerX
			} else {
				m.currentPlayer = PlayerO
			}
			m.winner = m.gameSession.Winner
			m.winningCells = m.gameSession.WinningCells
			m.isMyTurn = m.playerSymbol == m.currentPlayer
			m.waitingForPlayer = m.gameSession.PlayerCount < 2

			// Check for restart request
			if m.gameSession.RestartRequested {
				// Clear screen and reset local state to match shared state
				m.winner = m.gameSession.Winner
				m.winningCells = m.gameSession.WinningCells
				m.gameSession.mutex.RUnlock()
				return m, tea.ClearScreen
			}

			// Check for disconnect
			if m.gameSession.PlayerDisconnected {
				if m.disconnectTimer.IsZero() {
					m.disconnectTimer = time.Now()
				} else if time.Since(m.disconnectTimer) > DisconnectTimeout {
					// Disconnect timeout reached, quit the game
					m.gameSession.mutex.RUnlock()
					return m, tea.Quit
				}
			}
			m.gameSession.mutex.RUnlock()
		}

		// Continue ticking
		return m, tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})

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
			// In multiplayer, mark restart requested for other player
			if m.gameSession != nil {
				m.gameSession.mutex.Lock()
				m.gameSession.RestartRequested = true
				m.gameSession.mutex.Unlock()
			}
			return m, tea.ClearScreen

		// the "enter" and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			// ignore moves if the game is already over
			if m.winner != Empty {
				break
			}

			// only allow moves on your turn in multiplayer
			if m.gameSession != nil && !m.isMyTurn {
				break
			}

			// only place on empty cells
			if m.board[m.cursorY][m.cursorX] != Empty {
				break
			}

			// place the move - use player's symbol, not current player
			if m.gameSession != nil {
				m.board[m.cursorY][m.cursorX] = m.playerSymbol
			} else {
				m.board[m.cursorY][m.cursorX] = m.currentPlayer
			}

			// Update shared session if in multiplayer mode
			if m.gameSession != nil {
				m.gameSession.mutex.Lock()
				m.gameSession.Board[m.cursorY][m.cursorX] = m.playerSymbol

				cells := checkWinner(m.gameSession.Board, m.playerSymbol)
				if cells != nil {
					m.gameSession.Winner = m.playerSymbol
					m.gameSession.WinningCells = cells
				} else if isDraw(m.gameSession.Board) {
					m.gameSession.Winner = Draw
				} else {
					// Switch to next player
					m.gameSession.CurrentPlayer = 1 - m.gameSession.CurrentPlayer
				}
				m.gameSession.mutex.Unlock()
			} else {
				// Single player mode
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
	if m.gameSession != nil && m.gameSession.PlayerDisconnected {
		s += "\n" + lip.NewStyle().Foreground(lip.Color("#FF5555")).Bold(true).Render("⚠️  Opponent disconnected! Game will end in 5 seconds...") + "\n"
	} else if m.waitingForPlayer {
		s += "\n" + lip.NewStyle().Foreground(lip.Color("#FFB86C")).Bold(true).Render("Waiting for another player to join...") + "\n"
	} else if m.isMyTurn {
		s += footerStyle.Render("\nYour turn: ") + styledPlayer(m.currentPlayer) + "\n"
	} else {
		s += footerStyle.Render("\nOpponent's turn: ") + styledPlayer(m.currentPlayer) + "\n"
	}
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

// SSH handler - sets up multiplayer sessions
func handleSSHSession(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	model := initialModel()

	// Set up player based on session manager
	sessionManager.mutex.Lock()
	if sessionManager.waitingSession == nil {
		// First player
		model.playerSymbol = PlayerX
		model.isMyTurn = true
		model.waitingForPlayer = true

		// Create new session
		sessionManager.waitingSession = &GameSession{
			Board:         createEmptyBoard(),
			CurrentPlayer: 0,
			PlayerCount:   1,
		}
		model.gameSession = sessionManager.waitingSession
	} else {
		// Second player
		model.playerSymbol = PlayerO
		model.isMyTurn = false
		model.waitingForPlayer = false

		// Join existing session
		sessionManager.waitingSession.PlayerCount = 2
		model.gameSession = sessionManager.waitingSession
		sessionManager.waitingSession = nil
	}
	sessionManager.mutex.Unlock()

	// Set up disconnect detection
	go func() {
		// Simple disconnect detection - if session ends, mark as disconnected
		<-s.Context().Done()
		if model.gameSession != nil {
			model.gameSession.mutex.Lock()
			model.gameSession.PlayerDisconnected = true
			model.gameSession.mutex.Unlock()
		}
	}()

	return model, []tea.ProgramOption{
		tea.WithInput(s),
		tea.WithOutput(s),
		tea.WithAltScreen(),
	}
}

func main() {
	// Check if we should run in SSH mode or standalone
	if len(os.Args) > 1 && os.Args[1] == "ssh" {
		// SSH server mode
		server, err := wish.NewServer(
			wish.WithAddress(":2222"),
			wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
				return true // Allow all connections
			}),
			wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
				return true // Allow all connections
			}),
			wish.WithMiddleware(
				bubbletea.Middleware(handleSSHSession),
			),
		)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println("Starting SSH Tic-Tac-Toe server on :2222")
		fmt.Println("Players can connect with: ssh -p 2222 localhost")
		fmt.Println("Note: Using temporary host keys (more secure)")

		if err := server.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
	} else {
		// Standalone mode - original working version
		p := tea.NewProgram(initialModel())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
			os.Exit(1)
		}
	}
}
