package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Depik400/agent-kaiten-proxy/internal/apperr"
)

const (
	Version = 1
	EnvKey  = "KAITEN_PROXY_CONFIG"
)

var (
	hostNameRE = regexp.MustCompile(`^[A-Za-z]{1,100}$`)
	aliasRE    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
)

type Config struct {
	Version     int     `json:"version"`
	DefaultHost string  `json:"default_host,omitempty"`
	Hosts       []Host  `json:"hosts"`
	Aliases     Aliases `json:"aliases"`
}

type Host struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

type Aliases struct {
	Spaces map[string]SpaceAlias `json:"spaces,omitempty"`
	Boards map[string]BoardAlias `json:"boards,omitempty"`
	Lanes  map[string]LaneAlias  `json:"lanes,omitempty"`
}

type SpaceAlias struct {
	HostName string `json:"host_name,omitempty"`
	SpaceID  int    `json:"space_id"`
}

type BoardAlias struct {
	HostName string `json:"host_name,omitempty"`
	SpaceID  int    `json:"space_id,omitempty"`
	BoardID  int    `json:"board_id"`
}

type LaneAlias struct {
	HostName string `json:"host_name,omitempty"`
	BoardID  int    `json:"board_id"`
	LaneID   int    `json:"lane_id"`
}

func DefaultPath() (string, error) {
	if path := os.Getenv(EnvKey); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", apperr.Wrap(apperr.CodeConfig, "resolve user config dir", err, nil)
	}
	return filepath.Join(dir, "kaiten-proxy", "config.json"), nil
}

func Empty() Config {
	return Config{
		Version: Version,
		Hosts:   []Host{},
		Aliases: Aliases{
			Spaces: map[string]SpaceAlias{},
			Boards: map[string]BoardAlias{},
			Lanes:  map[string]LaneAlias{},
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, "read config", err, nil)
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, apperr.Wrap(apperr.CodeConfig, "parse config json", err, nil)
	}
	ensureAliases(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	ensureAliases(&cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return apperr.Wrap(apperr.CodeConfig, "create config dir", err, nil)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return apperr.Wrap(apperr.CodeConfig, "encode config", err, nil)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return apperr.Wrap(apperr.CodeConfig, "write config", err, nil)
	}
	return nil
}

func Validate(cfg Config) error {
	if cfg.Version != Version {
		return apperr.New(apperr.CodeConfig, fmt.Sprintf("unsupported config version %d", cfg.Version), nil)
	}
	seen := map[string]struct{}{}
	for _, host := range cfg.Hosts {
		if err := ValidateHost(host); err != nil {
			return err
		}
		if _, ok := seen[host.Name]; ok {
			return apperr.New(apperr.CodeConfig, "duplicate host name", map[string]string{"name": host.Name})
		}
		seen[host.Name] = struct{}{}
	}
	if cfg.DefaultHost != "" {
		if _, err := FindHost(cfg, cfg.DefaultHost); err != nil {
			return apperr.New(apperr.CodeConfig, "default host is not configured", map[string]string{"default_host": cfg.DefaultHost})
		}
	}
	for alias, item := range cfg.Aliases.Spaces {
		if err := ValidateAlias(alias); err != nil {
			return err
		}
		if item.SpaceID <= 0 {
			return apperr.New(apperr.CodeConfig, "space alias requires positive space_id", map[string]string{"alias": alias})
		}
		if item.HostName != "" {
			if _, err := FindHost(cfg, item.HostName); err != nil {
				return err
			}
		}
	}
	for alias, item := range cfg.Aliases.Boards {
		if err := ValidateAlias(alias); err != nil {
			return err
		}
		if item.BoardID <= 0 {
			return apperr.New(apperr.CodeConfig, "board alias requires positive board_id", map[string]string{"alias": alias})
		}
		if item.SpaceID < 0 {
			return apperr.New(apperr.CodeConfig, "board alias has invalid space_id", map[string]string{"alias": alias})
		}
		if item.HostName != "" {
			if _, err := FindHost(cfg, item.HostName); err != nil {
				return err
			}
		}
	}
	for alias, item := range cfg.Aliases.Lanes {
		if err := ValidateAlias(alias); err != nil {
			return err
		}
		if item.BoardID <= 0 || item.LaneID <= 0 {
			return apperr.New(apperr.CodeConfig, "lane alias requires positive board_id and lane_id", map[string]string{"alias": alias})
		}
		if item.HostName != "" {
			if _, err := FindHost(cfg, item.HostName); err != nil {
				return err
			}
		}
	}
	return nil
}

func ValidateHost(host Host) error {
	if !hostNameRE.MatchString(host.Name) {
		return apperr.New(apperr.CodeInvalidArgs, "host name must contain only English letters and be at most 100 characters", map[string]string{"name": host.Name})
	}
	if strings.TrimSpace(host.Token) == "" {
		return apperr.New(apperr.CodeInvalidArgs, "token is required", nil)
	}
	normalized, err := NormalizeURL(host.URL)
	if err != nil {
		return err
	}
	if normalized != host.URL {
		return apperr.New(apperr.CodeInvalidArgs, "url must be normalized", map[string]string{"normalized_url": normalized})
	}
	return nil
}

func ValidateAlias(alias string) error {
	if !aliasRE.MatchString(alias) {
		return apperr.New(apperr.CodeInvalidArgs, "alias must match ^[A-Za-z0-9_-]{1,100}$", map[string]string{"alias": alias})
	}
	return nil
}

func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apperr.New(apperr.CodeInvalidArgs, "url is required", nil)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", apperr.Wrap(apperr.CodeInvalidArgs, "invalid url", err, map[string]string{"url": raw})
	}
	u.Path = strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{"/api/latest", "/api/v1"} {
		if strings.HasSuffix(u.Path, suffix) {
			u.Path = strings.TrimSuffix(u.Path, suffix)
		}
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func UpsertHost(cfg Config, host Host, makeDefault bool) Config {
	ensureAliases(&cfg)
	for i := range cfg.Hosts {
		if cfg.Hosts[i].Name == host.Name {
			cfg.Hosts[i] = host
			if makeDefault || cfg.DefaultHost == "" {
				cfg.DefaultHost = host.Name
			}
			return cfg
		}
	}
	cfg.Hosts = append(cfg.Hosts, host)
	if makeDefault || cfg.DefaultHost == "" {
		cfg.DefaultHost = host.Name
	}
	return cfg
}

func FindHost(cfg Config, name string) (Host, error) {
	for _, host := range cfg.Hosts {
		if host.Name == name {
			return host, nil
		}
	}
	return Host{}, apperr.New(apperr.CodeConfig, "host is not configured", map[string]string{"host_name": name})
}

func ResolveHost(cfg Config, requested string) (Host, error) {
	if requested != "" {
		return FindHost(cfg, requested)
	}
	if cfg.DefaultHost != "" {
		return FindHost(cfg, cfg.DefaultHost)
	}
	if len(cfg.Hosts) == 1 {
		return cfg.Hosts[0], nil
	}
	return Host{}, apperr.New(apperr.CodeConfig, "host name is required when default_host is not configured", nil)
}

func ResolveSpaceAlias(cfg Config, alias string) (SpaceAlias, error) {
	item, ok := cfg.Aliases.Spaces[alias]
	if !ok {
		return SpaceAlias{}, apperr.New(apperr.CodeConfig, "space alias is not configured", map[string]string{"alias": alias})
	}
	return item, nil
}

func ResolveBoardAlias(cfg Config, alias string) (BoardAlias, error) {
	item, ok := cfg.Aliases.Boards[alias]
	if !ok {
		return BoardAlias{}, apperr.New(apperr.CodeConfig, "board alias is not configured", map[string]string{"alias": alias})
	}
	return item, nil
}

func ResolveLaneAlias(cfg Config, alias string) (LaneAlias, error) {
	item, ok := cfg.Aliases.Lanes[alias]
	if !ok {
		return LaneAlias{}, apperr.New(apperr.CodeConfig, "lane alias is not configured", map[string]string{"alias": alias})
	}
	return item, nil
}

func Mask(cfg Config) Config {
	out := cfg
	out.Hosts = append([]Host(nil), cfg.Hosts...)
	for i := range out.Hosts {
		if out.Hosts[i].Token != "" {
			out.Hosts[i].Token = ""
		}
	}
	return out
}

func ensureAliases(cfg *Config) {
	if cfg.Aliases.Spaces == nil {
		cfg.Aliases.Spaces = map[string]SpaceAlias{}
	}
	if cfg.Aliases.Boards == nil {
		cfg.Aliases.Boards = map[string]BoardAlias{}
	}
	if cfg.Aliases.Lanes == nil {
		cfg.Aliases.Lanes = map[string]LaneAlias{}
	}
}
