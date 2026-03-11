package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/whtspc/droeftoeter/config"

	tea "github.com/charmbracelet/bubbletea"
)

// filterPrintable drops null bytes and non-printable runes from input.
func filterPrintable(runes []rune) string {
	var b strings.Builder
	for _, r := range runes {
		if r != 0 && unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type providerPreset struct {
	label    string
	provider string
	baseURL  string
	model    string
	needsKey bool
	needsURL bool
}

var providerPresets = []providerPreset{
	{"Groq (free)", "openai", "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile", true, false},
	{"Gemini (free)", "gemini", "", "gemini-2.0-flash", true, false},
	{"OpenAI-compatible", "openai", "", "", true, true},
	{"Anthropic", "anthropic", "", "claude-sonnet-4-20250514", true, false},
	{"Ollama (local)", "ollama", "http://localhost:11434", "llama3", false, true},
}

const (
	fieldProvider = 0
	fieldAPIKey   = 1
	fieldBaseURL  = 2
	fieldModel    = 3
	fieldCount    = 4
)

func (m *Model) initSetupFromConfig() {
	m.setupField = 0
	m.setupAPIKey = m.cfg.APIKey
	m.setupBaseURL = m.cfg.BaseURL
	m.setupModel = m.cfg.Model

	// Find matching provider preset
	m.setupProviderIdx = 0
	for i, p := range providerPresets {
		if p.provider == m.cfg.Provider {
			if p.provider == "openai" && p.baseURL != "" && m.cfg.BaseURL == p.baseURL {
				m.setupProviderIdx = i
				break
			} else if p.provider != "openai" {
				m.setupProviderIdx = i
				break
			} else if p.baseURL == "" {
				// Generic OpenAI-compatible
				m.setupProviderIdx = i
			}
		}
	}
}

func (m *Model) currentPreset() providerPreset {
	return providerPresets[m.setupProviderIdx]
}

func (m *Model) applyPresetDefaults() {
	p := m.currentPreset()
	m.setupBaseURL = p.baseURL
	m.setupModel = p.model
	m.setupAPIKey = ""
}

func (m *Model) handleSetupKey(msg tea.KeyMsg) {
	preset := m.currentPreset()

	switch msg.Type {
	case tea.KeyCtrlC:
		return

	case tea.KeyEscape:
		if config.Exists() {
			m.view = viewGrid
			m.setStatus("[config unchanged]")
		}
		return

	case tea.KeyTab, tea.KeyDown:
		m.setupField = (m.setupField + 1) % fieldCount
		// Skip fields that don't apply
		m.skipIrrelevantFields(1)
		return

	case tea.KeyShiftTab, tea.KeyUp:
		m.setupField = (m.setupField - 1 + fieldCount) % fieldCount
		m.skipIrrelevantFields(-1)
		return

	case tea.KeyLeft:
		if m.setupField == fieldProvider {
			m.setupProviderIdx = (m.setupProviderIdx - 1 + len(providerPresets)) % len(providerPresets)
			m.applyPresetDefaults()
		}
		return

	case tea.KeyRight:
		if m.setupField == fieldProvider {
			m.setupProviderIdx = (m.setupProviderIdx + 1) % len(providerPresets)
			m.applyPresetDefaults()
		}
		return

	case tea.KeyEnter:
		m.saveSetup()
		return

	case tea.KeyBackspace:
		switch m.setupField {
		case fieldAPIKey:
			if len(m.setupAPIKey) > 0 {
				m.setupAPIKey = m.setupAPIKey[:len(m.setupAPIKey)-1]
			}
		case fieldBaseURL:
			if len(m.setupBaseURL) > 0 {
				m.setupBaseURL = m.setupBaseURL[:len(m.setupBaseURL)-1]
			}
		case fieldModel:
			if len(m.setupModel) > 0 {
				m.setupModel = m.setupModel[:len(m.setupModel)-1]
			}
		}
		return

	case tea.KeyRunes:
		ch := filterPrintable(msg.Runes)
		if ch == "" {
			return
		}
		switch m.setupField {
		case fieldAPIKey:
			if preset.needsKey {
				m.setupAPIKey += ch
			}
		case fieldBaseURL:
			if preset.needsURL {
				m.setupBaseURL += ch
			}
		case fieldModel:
			m.setupModel += ch
		}
		return
	}
}

func (m *Model) skipIrrelevantFields(dir int) {
	preset := m.currentPreset()
	for i := 0; i < fieldCount; i++ {
		if m.setupField == fieldAPIKey && !preset.needsKey {
			m.setupField = (m.setupField + dir + fieldCount) % fieldCount
		} else if m.setupField == fieldBaseURL && !preset.needsURL {
			m.setupField = (m.setupField + dir + fieldCount) % fieldCount
		} else {
			break
		}
	}
}

func (m *Model) saveSetup() {
	preset := m.currentPreset()

	if preset.needsKey && m.setupAPIKey == "" {
		m.setStatus("[error] API key is required")
		return
	}
	if preset.needsURL && m.setupBaseURL == "" {
		m.setStatus("[error] Base URL is required")
		return
	}
	if m.setupModel == "" {
		m.setStatus("[error] Model is required")
		return
	}

	m.cfg.Provider = preset.provider
	m.cfg.APIKey = m.setupAPIKey
	m.cfg.BaseURL = m.setupBaseURL
	m.cfg.Model = m.setupModel

	if err := config.Save(m.cfg); err != nil {
		m.setStatus("[error] " + err.Error())
		return
	}

	m.view = viewGrid
	m.setStatus(fmt.Sprintf("[config saved] %s / %s", preset.label, m.cfg.Model))
}

func (m Model) viewSetupScreen() string {
	var b strings.Builder
	preset := m.currentPreset()

	b.WriteString(headerStyle.Render("DROEFTOETER SETUP"))
	b.WriteString("\n\n")

	// Provider
	providerLine := fmt.Sprintf("  Provider:  %s %s %s",
		dimStyle.Render("<"),
		m.currentPreset().label,
		dimStyle.Render(">"))
	if m.setupField == fieldProvider {
		b.WriteString(promptStyle.Render("> "))
		b.WriteString(providerLine)
	} else {
		b.WriteString("  ")
		b.WriteString(providerLine)
	}
	b.WriteString("\n")

	// API Key
	if preset.needsKey {
		keyDisplay := m.setupAPIKey + "_"
		if m.setupField != fieldAPIKey && len(m.setupAPIKey) > 8 {
			keyDisplay = m.setupAPIKey[:6] + strings.Repeat("*", len(m.setupAPIKey)-8) + m.setupAPIKey[len(m.setupAPIKey)-2:]
		}
		if m.setupField == fieldAPIKey {
			b.WriteString(promptStyle.Render("> "))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("  API Key:   %s", keyDisplay))
		b.WriteString("\n")
	}

	// Base URL
	if preset.needsURL {
		urlDisplay := m.setupBaseURL
		if m.setupField == fieldBaseURL {
			urlDisplay += "_"
		}
		if m.setupField == fieldBaseURL {
			b.WriteString(promptStyle.Render("> "))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("  Base URL:  %s", urlDisplay))
		b.WriteString("\n")
	}

	// Model
	modelDisplay := m.setupModel
	if m.setupField == fieldModel {
		modelDisplay += "_"
	}
	if m.setupField == fieldModel {
		b.WriteString(promptStyle.Render("> "))
	} else {
		b.WriteString("  ")
	}
	b.WriteString(fmt.Sprintf("  Model:     %s", modelDisplay))
	b.WriteString("\n")

	// Status / help
	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(dimStyle.Render("  " + m.statusMsg))
		b.WriteString("\n")
	}
	b.WriteString(dimStyle.Render("  Tab/arrows to navigate, </> to change provider, Enter to save, Esc to cancel"))

	return b.String()
}
