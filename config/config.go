package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Provider string `toml:"provider"`
	APIKey   string `toml:"api_key"`
	BaseURL  string `toml:"base_url"`
	Model    string `toml:"model"`
}

const configPath = "config.toml"

func Exists() bool {
	_, err := os.Stat(configPath)
	return err == nil
}

func Save(cfg *Config) error {
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "provider = %q\napi_key  = %q\nbase_url = %q\nmodel    = %q\n",
		cfg.Provider, cfg.APIKey, cfg.BaseURL, cfg.Model)
	return err
}

func Load() *Config {
	cfg := &Config{
		Provider: "openai",
		BaseURL:  "https://api.groq.com/openai/v1",
		Model:    "llama-3.3-70b-versatile",
	}

	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		// Fall back to environment variables
		if v := os.Getenv("DROEFTOETER_PROVIDER"); v != "" {
			cfg.Provider = v
		}
		if v := os.Getenv("DROEFTOETER_API_KEY"); v != "" {
			cfg.APIKey = v
		}
		if v := os.Getenv("DROEFTOETER_BASE_URL"); v != "" {
			cfg.BaseURL = v
		} else if v := os.Getenv("DROEFTOETER_OLLAMA_URL"); v != "" {
			cfg.BaseURL = v
		}
		if v := os.Getenv("DROEFTOETER_MODEL"); v != "" {
			cfg.Model = v
		}
	}

	// Migrate legacy "groq" provider to generic "openai"
	if cfg.Provider == "groq" {
		cfg.Provider = "openai"
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.groq.com/openai/v1"
		}
	}

	return cfg
}
