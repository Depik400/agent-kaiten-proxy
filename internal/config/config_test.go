package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	got, err := NormalizeURL("https://example.kaiten.ru/api/latest/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.kaiten.ru" {
		t.Fatalf("NormalizeURL() = %q", got)
	}
}

func TestSaveLoadMaskAndResolveAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Empty()
	cfg = UpsertHost(cfg, Host{Name: "Main", URL: "https://example.kaiten.ru", Token: "secret"}, true)
	cfg.Aliases.Spaces["product"] = SpaceAlias{HostName: "Main", SpaceID: 1}
	cfg.Aliases.Boards["main"] = BoardAlias{HostName: "Main", SpaceID: 1, BoardID: 10}
	cfg.Aliases.Lanes["bugs"] = LaneAlias{HostName: "Main", BoardID: 10, LaneID: 22}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultHost != "Main" {
		t.Fatalf("DefaultHost = %q", loaded.DefaultHost)
	}
	if loaded.Aliases.Lanes["bugs"].LaneID != 22 {
		t.Fatalf("lane alias not loaded: %#v", loaded.Aliases.Lanes["bugs"])
	}
	masked := Mask(loaded)
	if masked.Hosts[0].Token != "" {
		t.Fatal("masked config leaked token")
	}
	host, err := ResolveHost(loaded, "")
	if err != nil {
		t.Fatal(err)
	}
	if host.Name != "Main" {
		t.Fatalf("ResolveHost() = %q", host.Name)
	}
}

func TestDefaultPathEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.json")
	t.Setenv(EnvKey, path)
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("DefaultPath() = %q", got)
	}
}

func TestValidateAlias(t *testing.T) {
	if err := ValidateAlias("bugs_1-main"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAlias("bad alias"); err == nil {
		t.Fatal("expected invalid alias error")
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != Version || len(cfg.Hosts) != 0 {
		t.Fatalf("unexpected empty config: %#v", cfg)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
