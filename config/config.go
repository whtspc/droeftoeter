package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Provider string `toml:"provider"`
	APIKey   string `toml:"api_key"`
	BaseURL  string `toml:"base_url"`
	Model    string `toml:"model"`
}

type fileFormat struct {
	Active   string            `toml:"active"`
	Profiles map[string]Config `toml:"profiles"`

	// Legacy flat fields for old config.toml migration
	Provider string `toml:"provider"`
	APIKey   string `toml:"api_key"`
	BaseURL  string `toml:"base_url"`
	Model    string `toml:"model"`
}

const configPath = "config.toml"

// profiles is the in-memory store of all saved profiles.
var profiles map[string]Config
var activeName string

func Exists() bool {
	_, err := os.Stat(configPath)
	return err == nil
}

func Save(cfg *Config, name string) error {
	if profiles == nil {
		profiles = make(map[string]Config)
	}
	profiles[name] = *cfg
	activeName = name
	return writeFile()
}

func writeFile() error {
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "active = %q\n\n", activeName)
	for name, p := range profiles {
		fmt.Fprintf(f, "[profiles.%s]\n", name)
		fmt.Fprintf(f, "provider = %q\n", p.Provider)
		fmt.Fprintf(f, "api_key  = %q\n", p.APIKey)
		fmt.Fprintf(f, "base_url = %q\n", p.BaseURL)
		fmt.Fprintf(f, "model    = %q\n\n", p.Model)
	}
	return nil
}

// ProfileNames returns all saved profile names.
func ProfileNames() []string {
	var names []string
	for name := range profiles {
		names = append(names, name)
	}
	return names
}

// ActiveName returns the current active profile name.
func ActiveName() string {
	return activeName
}

// Switch changes the active profile and returns it. Returns nil if not found.
func Switch(name string) *Config {
	p, ok := profiles[name]
	if !ok {
		return nil
	}
	activeName = name
	sanitize(&p)
	writeFile()
	return &p
}

// NextProfile cycles to the next profile and returns its name and config.
func NextProfile(current string) (string, *Config) {
	names := ProfileNames()
	if len(names) == 0 {
		return "", nil
	}
	// Find current index
	idx := 0
	for i, n := range names {
		if n == current {
			idx = (i + 1) % len(names)
			break
		}
	}
	name := names[idx]
	cfg := profiles[name]
	activeName = name
	sanitize(&cfg)
	writeFile()
	return name, &cfg
}

func Load() *Config {
	cfg := &Config{
		Provider: "openai",
		BaseURL:  "https://api.groq.com/openai/v1",
		Model:    "llama-3.3-70b-versatile",
	}

	var ff fileFormat
	if _, err := toml.DecodeFile(configPath, &ff); err != nil {
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
		profiles = make(map[string]Config)
		sanitize(cfg)
		return cfg
	}

	// New multi-profile format
	if ff.Profiles != nil && len(ff.Profiles) > 0 {
		profiles = ff.Profiles
		activeName = ff.Active
		if p, ok := profiles[activeName]; ok {
			*cfg = p
		} else {
			// Active name missing — pick first
			for name, p := range profiles {
				activeName = name
				*cfg = p
				break
			}
		}
	} else if ff.Provider != "" {
		// Legacy flat format — migrate
		cfg.Provider = ff.Provider
		cfg.APIKey = ff.APIKey
		cfg.BaseURL = ff.BaseURL
		cfg.Model = ff.Model
		profiles = map[string]Config{"default": *cfg}
		activeName = "default"
	} else {
		profiles = make(map[string]Config)
	}

	sanitize(cfg)

	// Migrate legacy "groq" provider to generic "openai"
	if cfg.Provider == "groq" {
		cfg.Provider = "openai"
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.groq.com/openai/v1"
		}
	}

	return cfg
}

func sanitize(cfg *Config) {
	cfg.Provider = strings.ReplaceAll(cfg.Provider, "\x00", "")
	cfg.APIKey = strings.ReplaceAll(cfg.APIKey, "\x00", "")
	cfg.BaseURL = strings.ReplaceAll(cfg.BaseURL, "\x00", "")
	cfg.Model = strings.ReplaceAll(cfg.Model, "\x00", "")
}
