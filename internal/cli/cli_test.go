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
	code := Run([]string{"install-skill", "--target-dir", target}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{
		"kaiten-proxy": "---\nname: kaiten-proxy\ndescription: test\n---\n",
	})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(target, "kaiten-proxy", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallSkillMultiple(t *testing.T) {
	target := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install-skill", "--target-dir", target}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{
		"kaiten-proxy":         "---\nname: kaiten-proxy\n---\n",
		"kaiten-card-history":  "---\nname: kaiten-card-history\n---\n",
		"kaiten-card-comments": "---\nname: kaiten-card-comments\n---\n",
	})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, name := range []string{"kaiten-proxy", "kaiten-card-history", "kaiten-card-comments"} {
		if _, err := os.Stat(filepath.Join(target, name, "SKILL.md")); err != nil {
			t.Fatalf("missing skill %s: %v", name, err)
		}
	}
}

func TestInstallSkillDefaultsToCodexAndClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install-skill"}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{
		"kaiten-card-edit": "---\nname: kaiten-card-edit\n---\n",
	})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, dir := range []string{".codex", ".claude"} {
		p := filepath.Join(home, dir, "skills", "kaiten-card-edit", "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
}

func TestInstallSkillClaudeOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"install-skill", "--claude"}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{
		"kaiten-proxy": "---\nname: kaiten-proxy\n---\n",
	})
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "kaiten-proxy", "SKILL.md")); err != nil {
		t.Fatalf("claude skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "kaiten-proxy", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("codex skill should not exist: %v", err)
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

func TestCardIncludeCommentsAndHistory(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/latest/cards/123":
			return cliJSONResponse(http.StatusOK, `{"id":123,"title":"card","description":"desc"}`), nil
		case "/api/latest/cards/123/comments":
			return cliJSONResponse(http.StatusOK, `[{"id":1,"text":"approved","author":{"id":1,"full_name":"Me"}}]`), nil
		case "/api/latest/cards/123/location-history":
			return cliJSONResponse(http.StatusOK, `[{"id":5,"lane_id":22,"author_id":1,"changed":"2024-01-01T00:00:00Z"}]`), nil
		case "/api/latest/cards/123/baselines":
			return cliJSONResponse(http.StatusOK, `[{"id":123,"planned_start":"2024-03-01T09:00:00.000Z"}]`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"card", "--id", "123", "--include-comments", "--include-history"})
	var data map[string]json.RawMessage
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"card", "comments", "history"} {
		if len(data[key]) == 0 {
			t.Fatalf("output missing %q: %s", key, out)
		}
	}
	if !bytes.Contains(out, []byte("location_history")) || !bytes.Contains(out, []byte("baselines")) {
		t.Fatalf("history missing parts: %s", out)
	}
}

func TestCardComments(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/latest/cards/7/comments" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `[{"id":1,"text":"remark","author":{"id":1,"full_name":"Me"}}]`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"card-comments", "--id", "7"})
	if !bytes.Contains(out, []byte(`"remark"`)) {
		t.Fatalf("comments output: %s", out)
	}
}

func TestCardHistory(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/latest/cards/7/location-history":
			return cliJSONResponse(http.StatusOK, `[{"id":5,"lane_id":22,"changed":"2024-01-01T00:00:00Z"}]`), nil
		case "/api/latest/cards/7/baselines":
			return cliJSONResponse(http.StatusOK, `[{"id":7,"planned_start":"2024-03-01T09:00:00.000Z"}]`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"card-history", "--id", "7"})
	if !bytes.Contains(out, []byte("location_history")) || !bytes.Contains(out, []byte("baselines")) {
		t.Fatalf("history output: %s", out)
	}
}

func TestColumns(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/latest/boards/10/columns" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `[{"id":7,"title":"TO DO"},{"id":8,"title":"Done"}]`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"columns", "--board", "main"})
	if !bytes.Contains(out, []byte(`"TO DO"`)) {
		t.Fatalf("columns output: %s", out)
	}
}

func TestCreateCardResolvesLaneAndColumnByName(t *testing.T) {
	var body map[string]any
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/latest/boards/10/lanes":
			return cliJSONResponse(http.StatusOK, `[{"id":22,"title":"Павел Кононов"},{"id":23,"title":"Другой"}]`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/api/latest/boards/10/columns":
			return cliJSONResponse(http.StatusOK, `[{"id":7,"title":"Queue","type":2,"subcolumns":[{"id":9,"title":"TO DO"}]},{"id":8,"title":"Done"}]`), nil
		case r.Method == http.MethodPost && r.URL.Path == "/api/latest/cards":
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatalf("bad request body: %v", err)
			}
			return cliJSONResponse(http.StatusOK, `{"id":555,"title":"New"}`), nil
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"create-card", "--board-id", "10", "--lane-name", "павел кононов", "--column-name", "TO DO", "--title", "New"})
	if !bytes.Contains(out, []byte(`555`)) {
		t.Fatalf("create-card output: %s", out)
	}
	if body["lane_id"] != float64(22) || body["column_id"] != float64(9) || body["board_id"] != float64(10) {
		t.Fatalf("resolved body = %#v", body)
	}
}

func TestCreateCardAmbiguousColumnName(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/latest/boards/10/lanes":
			return cliJSONResponse(http.StatusOK, `[{"id":22,"title":"Lane"}]`), nil
		case "/api/latest/boards/10/columns":
			return cliJSONResponse(http.StatusOK, `[{"id":7,"title":"In progress"},{"id":8,"title":"In review"}]`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"create-card", "--board-id", "10", "--lane-name", "Lane", "--column-name", "In", "--title", "x"}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{})
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("ambiguous")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestUpdateCardMovesByColumnNameOnCurrentBoard(t *testing.T) {
	var body map[string]any
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/latest/cards/123":
			return cliJSONResponse(http.StatusOK, `{"id":123,"board_id":10}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/api/latest/boards/10/columns":
			return cliJSONResponse(http.StatusOK, `[{"id":7,"title":"TO DO"},{"id":8,"title":"Done"}]`), nil
		case r.Method == http.MethodPatch && r.URL.Path == "/api/latest/cards/123":
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatalf("bad body: %v", err)
			}
			return cliJSONResponse(http.StatusOK, `{"id":123}`), nil
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	runOK(t, []string{"update-card", "--id", "123", "--column-name", "Done"})
	if body["column_id"] != float64(8) {
		t.Fatalf("patch body = %#v", body)
	}
	if _, ok := body["board_id"]; ok {
		t.Fatalf("board_id should not be sent when unchanged: %#v", body)
	}
}

func TestCreateCardRejectsLaneNameWithLaneID(t *testing.T) {
	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"create-card", "--board-id", "10", "--lane-id", "22", "--lane-name", "x", "--title", "t"}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{})
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestCardFiles(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/latest/cards/50/files" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return cliJSONResponse(http.StatusOK, `[{"id":9,"name":"notes.md","url":"https://files.example/notes"}]`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"card-files", "--id", "50"})
	if !bytes.Contains(out, []byte(`"notes.md"`)) {
		t.Fatalf("card-files output: %s", out)
	}
}

func TestAttachFileUploadsText(t *testing.T) {
	var gotName, gotBody string
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/latest/cards/50/files" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		fh := r.MultipartForm.File["file"][0]
		gotName = fh.Filename
		f, _ := fh.Open()
		data, _ := io.ReadAll(f)
		gotBody = string(data)
		return cliJSONResponse(http.StatusOK, `{"id":9,"name":"notes.md"}`), nil
	})
	defer restore()

	dir := t.TempDir()
	src := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(src, []byte("# hello\ntext body\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"attach-file", "--id", "50", "--file", src})
	if !bytes.Contains(out, []byte(`"id": 9`)) {
		t.Fatalf("attach-file output: %s", out)
	}
	if gotName != "notes.md" || gotBody != "# hello\ntext body\n" {
		t.Fatalf("uploaded name=%q body=%q", gotName, gotBody)
	}
}

func TestAttachFileRejectsBinary(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(src, []byte{0x00, 0x01, 0x02, 0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"attach-file", "--id", "50", "--file", src}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{})
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("only text files")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestReadFileByName(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/latest/cards/50/files":
			return cliJSONResponse(http.StatusOK, `[{"id":9,"name":"notes.md","url":"https://files.example/notes","size":11}]`), nil
		case r.URL.Host == "files.example" && r.URL.Path == "/notes":
			return cliJSONResponse(http.StatusOK, `hello world`), nil
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.String())
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	out := runOK(t, []string{"read-file", "--id", "50", "--name", "notes"})
	var data struct {
		Content string `json:"content"`
		Bytes   int    `json:"bytes"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatal(err)
	}
	if data.Content != "hello world" || data.Bytes != 11 {
		t.Fatalf("read-file output: %s", out)
	}
}

func TestReadFileRejectsBinaryContent(t *testing.T) {
	restore := stubClient(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/latest/cards/50/files":
			return cliJSONResponse(http.StatusOK, `[{"id":9,"name":"blob.bin","url":"https://files.example/blob"}]`), nil
		case r.URL.Host == "files.example":
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader([]byte{0x00, 0x01, 0x02}))}, nil
		default:
			t.Fatalf("unexpected %s", r.URL.String())
		}
		return cliJSONResponse(http.StatusOK, `{}`), nil
	})
	defer restore()

	path := writeTestConfig(t, "https://kaiten.test")
	t.Setenv(config.EnvKey, path)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"read-file", "--id", "50", "--file-id", "9"}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{})
	if code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestUpdateCardRequiresField(t *testing.T) {
	path := writeTestConfig(t, "https://example.kaiten.ru")
	t.Setenv(config.EnvKey, path)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"update-card", "--id", "1"}, bytes.NewReader(nil), &stdout, &stderr, map[string]string{})
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
	code := Run(args, bytes.NewReader(nil), &stdout, &stderr, map[string]string{"kaiten-proxy": "skill"})
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
