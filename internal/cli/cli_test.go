package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Depik400/agent-kaiten-proxy/internal/config"
	"github.com/Depik400/agent-kaiten-proxy/internal/kaiten"
)

func TestInstallSkill(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install-skill", "--target-dir", target}, bytes.NewReader(nil), &stdout, &stderr, "---\nname: kaiten-proxy\ndescription: test\n---\n")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(target, "kaiten-proxy", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestAliasCRUD(t *testing.T) {
	path := writeTestConfig(t, "https://example.kaiten.ru")
	t.Setenv(config.EnvKey, path)

	runOK(t, []string{"alias-space", "set", "--alias", "product", "--space-id", "1"})
	runOK(t, []string{"alias-board", "set", "--alias", "main", "--space-id", "1", "--board-id", "10"})
	runOK(t, []string{"alias-lane", "set", "--alias", "bugs", "--board-id", "10", "--lane-id", "22"})
	out := runOK(t, []string{"aliases"})
	if !bytes.Contains(out, []byte(`"bugs"`)) {
		t.Fatalf("aliases output missing bugs: %s", out)
	}
	runOK(t, []string{"alias-lane", "remove", "--alias", "bugs"})
	out = runOK(t, []string{"aliases"})
	if bytes.Contains(out, []byte(`"bugs"`)) {
		t.Fatalf("aliases output still has bugs: %s", out)
	}
}

func TestLaneCardsDetails(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/latest/cards":
			if r.URL.Query().Get("board_id") != "10" || r.URL.Query().Get("lane_id") != "22" {
				t.Fatalf("query = %s", r.URL.RawQuery)
			}
			return cliJSONResponse(http.StatusOK, `[{"id":123,"title":"short"}]`), nil
		case "/api/latest/cards/123":
			return cliJSONResponse(http.StatusOK, `{"id":123,"title":"full","description":"details","owner":{"id":1}}`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"lane-cards", "--lane", "bugs", "--details", "--include-description"})
	if !bytes.Contains(out, []byte(`"full"`)) {
		t.Fatalf("expected detailed card: %s", out)
	}
}

func TestMyCardsDedup(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/latest/users/current":
			return cliJSONResponse(http.StatusOK, `{"id":7,"username":"me"}`), nil
		case "/api/latest/cards":
			return cliJSONResponse(http.StatusOK, `[{"id":1,"title":"same"}]`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"my-cards"})
	var cards []map[string]any
	if err := json.Unmarshal(out, &cards); err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards)=%d output=%s", len(cards), out)
	}
}

func TestUpdateCardRequiresField(t *testing.T) {
	path := writeTestConfig(t, "https://example.kaiten.ru")
	t.Setenv(config.EnvKey, path)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"update-card", "--id", "1"}, bytes.NewReader(nil), &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func writeTestConfig(t *testing.T, url string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Empty()
	cfg = config.UpsertHost(cfg, config.Host{Name: "Main", URL: url, Token: "token"}, true)
	cfg.Aliases.Spaces["product"] = config.SpaceAlias{HostName: "Main", SpaceID: 1}
	cfg.Aliases.Boards["main"] = config.BoardAlias{HostName: "Main", SpaceID: 1, BoardID: 10}
	cfg.Aliases.Lanes["bugs"] = config.LaneAlias{HostName: "Main", BoardID: 10, LaneID: 22}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	return path
}

func runOK(t *testing.T, args []string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, bytes.NewReader(nil), &stdout, &stderr, "skill")
	if code != 0 {
		t.Fatalf("%v: code=%d stderr=%s", args, code, stderr.String())
	}
	return stdout.Bytes()
}

func stubClient(fn func(*http.Request) (*http.Response, error)) func() {
	old := newKaitenClient
	newKaitenClient = func(baseURL, token string) *kaiten.Client {
		return kaiten.NewClientWithHTTP(baseURL, token, &http.Client{Transport: cliRoundTripFunc(fn)})
	}
	return func() {
		newKaitenClient = old
	}
}

type cliRoundTripFunc func(*http.Request) (*http.Response, error)

func (f cliRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func cliJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
