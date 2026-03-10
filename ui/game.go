package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/whtspc/droeftoeter/config"
	"github.com/whtspc/droeftoeter/llm"
	"github.com/whtspc/droeftoeter/sandbox"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Grid dimensions come from sandbox.GridW / sandbox.GridH

// Styles
var (
	gridStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)
)

// Messages
type tickMsg int

type llmResultMsg struct {
	code string
	err  error
}

// View modes
type viewMode int

const (
	viewGrid viewMode = iota
	viewCode
	viewHistory
	viewSetup
)

type codeEntry struct {
	prompt string
	code   string
}

type Model struct {
	sb       *sandbox.Sandbox
	cfg      *config.Config
	tickNum  int

	// Input
	input     string
	statusMsg string

	// LLM
	thinking    bool
	currentCode string
	promptHistory []string
	codeHistory   []codeEntry

	// View
	view       viewMode
	scrollOffset int
	scrollMax    int

	// Size
	width  int
	height int

	// Setup screen
	setupField       int // 0=provider, 1=apikey, 2=baseurl, 3=model
	setupProviderIdx int
	setupAPIKey      string
	setupBaseURL     string
	setupModel       string
}

func NewModel(cfg *config.Config) Model {
	m := Model{
		cfg: cfg,
	}

	m.sb = sandbox.New(func(msg string) {
		m.setStatus(msg)
	})

	if !config.Exists() {
		m.view = viewSetup
		m.initSetupFromConfig()
		m.setStatus("Welcome! Configure your LLM provider to get started.")
	} else {
		m.setStatus("Droeftoeter ready. Type /help for commands.")
	}

	return m
}

func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
}

func tickCmd() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(0)
	})
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.tickNum++
		m.sb.RunTick(m.tickNum)
		return m, tickCmd()

	case llmResultMsg:
		m.thinking = false
		if msg.err != nil {
			m.setStatus("[error] " + msg.err.Error())
		} else {
			m.sb.Reset()
			m.sb.Inject(msg.code)
			m.currentCode = msg.code
			m.setStatus("[code updated]")
		}
		return m, nil

	case tea.KeyMsg:
		if m.view == viewSetup {
			m.handleSetupKey(msg)
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEscape:
			if m.view != viewGrid {
				m.view = viewGrid
				m.scrollOffset = 0
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyEnter:
			if m.view != viewGrid {
				return m, nil
			}
			if len(m.input) > 0 {
				m.handleInput()
			}
			return m, nil

		case tea.KeyBackspace:
			if m.view == viewGrid && len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			return m, nil

		case tea.KeyUp:
			if m.view != viewGrid {
				m.scrollOffset -= 3
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}
			return m, nil

		case tea.KeyDown:
			if m.view != viewGrid {
				m.scrollOffset += 3
			}
			return m, nil

		case tea.KeyPgUp:
			if m.view != viewGrid {
				m.scrollOffset -= 15
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}
			return m, nil

		case tea.KeyPgDown:
			if m.view != viewGrid {
				m.scrollOffset += 15
			}
			return m, nil

		case tea.KeyRunes:
			if m.view == viewGrid {
				m.input += string(msg.Runes)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) handleInput() {
	input := m.input
	m.input = ""

	// Slash commands
	if strings.HasPrefix(input, "/") {
		m.handleCommand(input)
		return
	}

	// LLM request
	m.thinking = true
	m.setStatus("[thinking...]")

	go func() {
		var userMsg strings.Builder
		if len(m.promptHistory) > 0 {
			userMsg.WriteString("Previous requests (for context):\n")
			for i, p := range m.promptHistory {
				userMsg.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
			}
			userMsg.WriteString("\nNew request: ")
		}
		userMsg.WriteString(input)

		systemPrompt := llm.BuildSystemPrompt(m.currentCode)

		messages := []llm.Message{
			{Role: "user", Content: userMsg.String()},
		}

		llmCfg := &llm.Config{
			Provider: m.cfg.Provider,
			APIKey:   m.cfg.APIKey,
			BaseURL:  m.cfg.BaseURL,
			Model:    m.cfg.Model,
		}

		result, err := llm.Generate(llmCfg, systemPrompt, messages)

		m.promptHistory = append(m.promptHistory, input)
		if err == nil {
			m.codeHistory = append(m.codeHistory, codeEntry{prompt: input, code: result})
		}

		// Send result back via the program
		if programPtr != nil {
			programPtr.Send(llmResultMsg{code: result, err: err})
		}
	}()
}

// programPtr is set by Run() so goroutines can send messages back
var programPtr *tea.Program

func (m *Model) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	switch parts[0] {
	case "/help":
		m.setStatus("/rerun /clear /code /history /config /export-code /export-prompt")

	case "/rerun":
		if m.currentCode == "" {
			m.setStatus("[no code to rerun]")
			return
		}
		m.sb.Reset()
		m.tickNum = 0
		m.sb.Inject(m.currentCode)
		m.setStatus("[rerun]")

	case "/clear":
		m.sb.Reset()
		m.currentCode = ""
		m.tickNum = 0
		m.setStatus("[cleared]")

	case "/code":
		if m.currentCode == "" {
			m.setStatus("[no code yet]")
			return
		}
		m.view = viewCode
		m.scrollOffset = 0

	case "/history":
		if len(m.codeHistory) == 0 {
			m.setStatus("[no history yet]")
			return
		}
		m.view = viewHistory
		m.scrollOffset = 0

	case "/config":
		m.view = viewSetup
		m.initSetupFromConfig()

	case "/export-code":
		m.exportCode()

	case "/export-prompt":
		m.exportPrompt()

	default:
		m.setStatus("[unknown command: " + parts[0] + "] type /help for commands")
	}
}

func (m *Model) exportCode() {
	if m.currentCode == "" {
		m.setStatus("[no code to export]")
		return
	}
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("droeftoeter-code-%s.js", ts)

	var sb strings.Builder
	sb.WriteString("// === Current Running Code ===\n")
	sb.WriteString(m.currentCode)
	sb.WriteString("\n\n// === History ===\n")
	for i, entry := range m.codeHistory {
		sb.WriteString(fmt.Sprintf("\n// --- #%d Prompt: %s ---\n", i+1, entry.prompt))
		sb.WriteString(entry.code)
		sb.WriteString("\n")
	}

	if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
		m.setStatus("[export error] " + err.Error())
		return
	}
	m.setStatus("[exported] " + filename)
}

func (m *Model) exportPrompt() {
	systemPrompt := llm.BuildSystemPrompt(m.currentCode)

	var prompt strings.Builder
	prompt.WriteString(systemPrompt)
	if len(m.promptHistory) > 0 {
		prompt.WriteString("\n\n--- USER PROMPT HISTORY ---\n")
		for i, p := range m.promptHistory {
			prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
		}
	}

	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("droeftoeter-prompt-%s.txt", ts)

	if err := os.WriteFile(filename, []byte(prompt.String()), 0644); err != nil {
		m.setStatus("[export error] " + err.Error())
		return
	}
	m.setStatus("[exported] " + filename)
}

func (m Model) View() string {
	switch m.view {
	case viewSetup:
		return m.viewSetupScreen()
	case viewCode:
		return m.viewCodeScreen()
	case viewHistory:
		return m.viewHistoryScreen()
	default:
		return m.viewGridScreen()
	}
}

func (m Model) viewGridScreen() string {
	var b strings.Builder

	// Render grid
	grid := m.sb.GetGrid()
	var gridLines []string
	for y := 0; y < sandbox.GridH; y++ {
		var row strings.Builder
		for x := 0; x < sandbox.GridW; x++ {
			cell := grid[x][y]
			if cell == nil {
				row.WriteRune(' ')
			} else {
				ch := cell.Char
				if ch == "" {
					ch = "?"
				}
				if cell.Color != "" {
					row.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color(cell.Color)).
						Render(string(ch[0])))
				} else {
					row.WriteString(string(ch[0]))
				}
			}
		}
		gridLines = append(gridLines, row.String())
	}
	gridContent := strings.Join(gridLines, "\n")
	b.WriteString(gridStyle.Render(gridContent))
	b.WriteString("\n")

	// Status line
	if m.statusMsg != "" {
		b.WriteString(dimStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	// Prompt
	b.WriteString(promptStyle.Render("> ") + m.input + promptStyle.Render("_"))

	return b.String()
}

func (m Model) viewCodeScreen() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("=== CURRENT CODE === (Esc to return, PgUp/PgDown to scroll)"))
	b.WriteString("\n\n")

	lines := strings.Split(m.currentCode, "\n")
	maxVisible := m.height - 4
	if maxVisible < 5 {
		maxVisible = 20
	}

	// Clamp scroll
	maxScroll := len(lines) - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}

	end := m.scrollOffset + maxVisible
	if end > len(lines) {
		end = len(lines)
	}

	for i, line := range lines[m.scrollOffset:end] {
		lineNum := m.scrollOffset + i + 1
		b.WriteString(dimStyle.Render(fmt.Sprintf("%3d ", lineNum)))
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("\n%s", dimStyle.Render(fmt.Sprintf("[lines %d-%d of %d]", m.scrollOffset+1, end, len(lines)))))

	return b.String()
}

func (m Model) viewHistoryScreen() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("=== CODE HISTORY === (Esc to return, PgUp/PgDown to scroll)"))
	b.WriteString("\n\n")

	var allLines []string
	for i, entry := range m.codeHistory {
		allLines = append(allLines, headerStyle.Render(fmt.Sprintf("--- #%d: %s ---", i+1, entry.prompt)))
		for _, cl := range strings.Split(entry.code, "\n") {
			allLines = append(allLines, cl)
		}
		allLines = append(allLines, "")
	}

	maxVisible := m.height - 4
	if maxVisible < 5 {
		maxVisible = 20
	}

	maxScroll := len(allLines) - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}

	end := m.scrollOffset + maxVisible
	if end > len(allLines) {
		end = len(allLines)
	}

	for _, line := range allLines[m.scrollOffset:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("\n%s", dimStyle.Render(fmt.Sprintf("[lines %d-%d of %d]", m.scrollOffset+1, end, len(allLines)))))

	return b.String()
}

func Run(cfg *config.Config) error {
	m := NewModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	programPtr = p
	_, err := p.Run()
	return err
}
