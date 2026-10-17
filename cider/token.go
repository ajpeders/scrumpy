package cider

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrNoToken = errors.New("no Cider API token; create one in Cider Settings → Connectivity → Manage External Application Access")

// ErrNoCider means Cider's config file doesn't exist, which on a fresh
// machine usually means Cider isn't installed yet. It wraps ErrNoToken.
var ErrNoCider = fmt.Errorf("Cider's config not found: %w", ErrNoToken)

const DefaultURL = "http://localhost:10767"

// userConfigDir is the platform config directory resolver. The variable
// points at os.UserConfigDir by default; tests swap it to exercise the
// macOS / Windows code paths without leaving Linux.
var userConfigDir = os.UserConfigDir

// DiscoverToken returns the token and its source. A blank token is valid only
// when Cider's configuration explicitly disables the token requirement.
func DiscoverToken() (token, source string, err error) {
	if token := os.Getenv("CIDER_TOKEN"); token != "" {
		return token, "CIDER_TOKEN", nil
	}
	// Cider is Electron, so its config lives in the platform config directory:
	// $XDG_CONFIG_HOME (or ~/.config), ~/Library/Application Support, %AppData%.
	dir, err := userConfigDir()
	if err != nil {
		return "", "", ErrNoToken
	}
	return tokenFromFile(filepath.Join(dir, "sh.cider.genten", "spa-config.yml"))
}

func tokenFromFile(path string) (token, source string, err error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", ErrNoCider
	}
	if err != nil {
		return "", "", ErrNoToken
	}
	var config struct {
		Connectivity struct {
			APITokensRequired *bool `yaml:"apiTokensRequired"`
			APITokens         []struct {
				Token string `yaml:"token"`
			} `yaml:"apiTokens"`
		} `yaml:"connectivity"`
	}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return "", "", ErrNoToken
	}
	if config.Connectivity.APITokensRequired != nil && !*config.Connectivity.APITokensRequired {
		return "", "Cider config (tokens disabled)", nil
	}
	for _, entry := range config.Connectivity.APITokens {
		if strings.TrimSpace(entry.Token) != "" {
			return entry.Token, "Cider config", nil
		}
	}
	return "", "", ErrNoToken
}

// NewFromEnvironment uses CIDER_URL and discovers the token without exposing it
// in errors or status output.
func NewFromEnvironment() (*Client, string, error) {
	base := os.Getenv("CIDER_URL")
	if base == "" {
		base = DefaultURL
	}
	token, source, err := DiscoverToken()
	if err != nil {
		return nil, "", err
	}
	client, err := New(base, token)
	if err != nil {
		return nil, "", fmt.Errorf("invalid CIDER_URL: %w", err)
	}
	return client, source, nil
}
