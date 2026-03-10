# Droeftoeter

An LLM-powered ASCII grid world in your terminal. Describe what you want to see and an LLM writes JavaScript that brings a 64x32 character grid to life.

## Download

Grab a binary from the [Releases](https://github.com/whtspc/droeftoeter/releases) page — no Go installation needed.

## Build from source

```
go build -o droeftoeter .
```

Or use the Makefile to cross-compile for all platforms:

```
make all
```

## Usage

Run the binary. On first launch you'll be prompted to configure an LLM provider.

Supported providers:
- **Groq** (free) — fast inference with Llama models
- **Gemini** (free) — Google's Gemini API
- **OpenAI-compatible** — any OpenAI-compatible endpoint
- **Anthropic** — Claude models
- **Ollama** (local) — run models locally

You can also configure via environment variables:

```
DROEFTOETER_PROVIDER=openai
DROEFTOETER_API_KEY=your-key
DROEFTOETER_BASE_URL=https://api.groq.com/openai/v1
DROEFTOETER_MODEL=llama-3.3-70b-versatile
```

Or create a `config.toml` (see `config.toml.example`).

### Commands

- `/help` — list commands
- `/code` — view current running code
- `/history` — view code history
- `/rerun` — restart the current program
- `/clear` — clear the grid
- `/config` — reconfigure provider
- `/export-code` — save code to file
- `/export-prompt` — save system prompt to file

## License

MIT
