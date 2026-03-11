package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type Config struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var (
	fenceRe = regexp.MustCompile("(?s)^\\s*```(?:javascript|js)?\\s*\n?(.*?)\\s*```\\s*$")
	thinkRe = regexp.MustCompile("(?s)<think>.*?</think>")
)

func stripResponse(s string) string {
	s = thinkRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	if m := fenceRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return s
}

// Generate sends the system prompt plus full conversation history to the LLM.
func Generate(cfg *Config, systemPrompt string, history []Message) (string, error) {
	switch cfg.Provider {
	case "anthropic":
		return generateAnthropic(cfg, systemPrompt, history)
	case "openai":
		return generateOpenAI(cfg, systemPrompt, history)
	case "gemini":
		return generateGemini(cfg, systemPrompt, history)
	case "ollama":
		return generateOllama(cfg, systemPrompt, history)
	default:
		return "", fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

func generateAnthropic(cfg *Config, systemPrompt string, history []Message) (string, error) {
	messages := make([]map[string]string, len(history))
	for i, m := range history {
		messages[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	body := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages":   messages,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return stripResponse(result.Content[0].Text), nil
}

func generateOpenAI(cfg *Config, systemPrompt string, history []Message) (string, error) {
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, m := range history {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}

	body := map[string]interface{}{
		"model":      cfg.Model,
		"max_tokens": 4096,
		"messages":   messages,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return stripResponse(result.Choices[0].Message.Content), nil
}

func generateGemini(cfg *Config, systemPrompt string, history []Message) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		cfg.Model, cfg.APIKey)

	var contents []map[string]interface{}
	for _, m := range history {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}

	body := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": systemPrompt}},
		},
		"contents": contents,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Gemini error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return stripResponse(result.Candidates[0].Content.Parts[0].Text), nil
}

func generateOllama(cfg *Config, systemPrompt string, history []Message) (string, error) {
	url := strings.TrimRight(cfg.BaseURL, "/") + "/api/chat"

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, m := range history {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}

	body := map[string]interface{}{
		"model":    cfg.Model,
		"messages": messages,
		"stream":   false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("Ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Ollama error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	return stripResponse(result.Message.Content), nil
}

func BuildSystemPrompt(currentCode string) string {
	codeSection := "(none — this is a blank world)"
	if currentCode != "" {
		codeSection = currentCode
	}

	return `You are a code generator for a 64x32 ASCII grid world rendered in a terminal.
You write a complete, self-contained JavaScript program that controls the grid.
Your code replaces the previous program entirely — it must be fully self-contained.

AVAILABLE API:
- grid              — 64x32 2D array. Read cells: grid[x][y] is null or {char, color}
                       x is column (0=left, 63=right), y is row (0=top, 31=bottom)
- setCell(x, y, char, color) — draw a character. x/y clamped to grid bounds.
                       char: single character. color: hex string e.g. "#ff0000"
                       setCell(x, y, null) clears the cell.
- clearGrid()       — set all cells to null
- onTick(fn)        — register a function called ~30x/sec. fn(tickNumber).
                       Only one handler; calling again replaces it.
- log(msg)          — print to the log panel
- gridW             — 64 (read only)
- gridH             — 32 (read only)

THINGS THAT DO NOT EXIST — do not use these:
- There is NO keyboard/mouse input. Do NOT use onKey(). There is no player control.
- There is no setTimeout, setInterval, fetch, require, console, or DOM.
- All interaction is autonomous — entities move and behave on their own via onTick.

CRITICAL — PRESERVING EXISTING CODE:
- The "CURRENT RUNNING CODE" below is the program currently running in the world.
- You MUST keep ALL existing code and behavior unless the user explicitly asks to change or remove it.
- ADD new features on top of the existing code. Do not rewrite from scratch.
- If there is existing code, copy it entirely and add/modify only what the user asks for.
- Only if the user says "clear", "reset", "start over", or "remove" should you drop existing code.

VISUAL STYLE:
- You have the full Unicode range available. Mix block elements (█ ▓ ▒ ░ ▄ ▀),
  box-drawing (─ │ ┌ ┐ └ ┘), symbols (● ◆ ★ ▲ ■), and regular characters creatively.
- Do NOT use emoji or other double-width characters — they break the grid alignment.

RULES:
- Respond with ONLY a complete JavaScript program. No explanation, no markdown, no backticks.
- Use setCell() for all drawing. Do NOT assign to grid[][] directly.
- No setTimeout/setInterval — use onTick with counters for timing.
- Use Math.random() for randomness.
- The program starts fresh each time — define all variables and state you need.

CURRENT RUNNING CODE (preserve and extend this — do NOT discard unless asked):
` + codeSection
}
