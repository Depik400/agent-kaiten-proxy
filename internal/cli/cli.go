package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Depik400/agent-kaiten-proxy/internal/apperr"
	"github.com/Depik400/agent-kaiten-proxy/internal/config"
	"github.com/Depik400/agent-kaiten-proxy/internal/kaiten"
	"github.com/Depik400/agent-kaiten-proxy/internal/output"
)

var newKaitenClient = kaiten.NewClient

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, skills map[string]string) int {
	if len(args) == 0 {
		writeHelp(stdout, "")
		return apperr.ExitOK
	}
	if err := run(args, stdin, stdout, skills); err != nil {
		output.Error(stderr, err)
		return apperr.ExitCode(err)
	}
	return apperr.ExitOK
}

func run(args []string, stdin io.Reader, stdout io.Writer, skills map[string]string) error {
	if args[0] == "--help" || args[0] == "-h" {
		writeHelp(stdout, "")
		return nil
	}
	if args[0] == "help" {
		topic := ""
		if len(args) > 2 {
			return apperr.New(apperr.CodeInvalidArgs, "usage: agent-kaiten-proxy help [command]", map[string][]string{"args": args[1:]})
		}
		if len(args) == 2 {
			topic = args[1]
		}
		return writeHelp(stdout, topic)
	}
	if hasHelpFlag(args[1:]) {
		return writeHelp(stdout, args[0])
	}
	switch args[0] {
	case "bootstrap":
		return runBootstrap(args[1:], stdin, stdout)
	case "config":
		return runConfig(args[1:], stdout)
	case "set-default":
		return runSetDefault(args[1:], stdout)
	case "whoami":
		return runWhoami(args[1:], stdout)
	case "aliases":
		return runAliases(args[1:], stdout)
	case "alias-space":
		return runAliasSpace(args[1:], stdout)
	case "alias-board":
		return runAliasBoard(args[1:], stdout)
	case "alias-lane":
		return runAliasLane(args[1:], stdout)
	case "my-cards":
		return runMyCards(args[1:], stdout)
	case "cards":
		return runCards(args[1:], stdout)
	case "space-cards":
		return runSpaceCards(args[1:], stdout)
	case "lane-cards":
		return runLaneCards(args[1:], stdout)
	case "card":
		return runCard(args[1:], stdout)
	case "card-comments":
		return runCardComments(args[1:], stdout)
	case "card-history":
		return runCardHistory(args[1:], stdout)
	case "spaces":
		return runSpaces(args[1:], stdout)
	case "boards":
		return runBoards(args[1:], stdout)
	case "lanes":
		return runLanes(args[1:], stdout)
	case "columns":
		return runColumns(args[1:], stdout)
	case "create-card":
		return runCreateCard(args[1:], stdout)
	case "update-card":
		return runUpdateCard(args[1:], stdout)
	case "comment-card":
		return runCommentCard(args[1:], stdout)
	case "card-files":
		return runCardFiles(args[1:], stdout)
	case "attach-file":
		return runAttachFile(args[1:], stdout)
	case "read-file":
		return runReadFile(args[1:], stdout)
	case "users":
		return runUsers(args[1:], stdout)
	case "card-members":
		return runCardMembers(args[1:], stdout)
	case "add-member":
		return runAddMember(args[1:], stdout)
	case "remove-member":
		return runRemoveMember(args[1:], stdout)
	case "install-skill":
		return runInstallSkill(args[1:], stdout, skills)
	default:
		return apperr.New(apperr.CodeInvalidArgs, "unknown command", map[string]string{"command": args[0]})
	}
}

func runBootstrap(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := newFlagSet("bootstrap")
	interactive := fs.Bool("interactive", false, "prompt for url, token and name")
	rawURL := fs.String("url", "", "Kaiten URL")
	token := fs.String("token", "", "Kaiten access token")
	name := fs.String("name", "", "host name")
	makeDefault := fs.Bool("default", false, "make this host default")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *interactive {
		values, err := promptBootstrap(stdin, stdout)
		if err != nil {
			return err
		}
		*rawURL = values.URL
		*token = values.Token
		*name = values.Name
	}
	if *rawURL == "" || *token == "" || *name == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--url, --token and --name are required unless --interactive is set", nil)
	}
	normalizedURL, err := config.NormalizeURL(*rawURL)
	if err != nil {
		return err
	}
	host := config.Host{Name: *name, URL: normalizedURL, Token: *token}
	if err := config.ValidateHost(host); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := kaiten.NewClient(host.URL, host.Token).VerifyToken(ctx); err != nil {
		return err
	}
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cfg = config.UpsertHost(cfg, host, *makeDefault)
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	return output.JSON(stdout, map[string]any{"status": "ok", "host": config.Host{Name: host.Name, URL: host.URL}, "default_host": cfg.DefaultHost})
}

func runConfig(args []string, stdout io.Writer) error {
	fs := newFlagSet("config")
	if err := parse(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return output.JSON(stdout, config.Mask(cfg))
}

func runSetDefault(args []string, stdout io.Writer) error {
	fs := newFlagSet("set-default")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *hostName == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--host-name is required", nil)
	}
	path, cfg, err := loadConfigWithPath()
	if err != nil {
		return err
	}
	if _, err := config.FindHost(cfg, *hostName); err != nil {
		return err
	}
	cfg.DefaultHost = *hostName
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	return output.JSON(stdout, map[string]any{"status": "ok", "default_host": cfg.DefaultHost})
}

func runWhoami(args []string, stdout io.Writer) error {
	fs := newFlagSet("whoami")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	return output.JSON(stdout, user)
}

func runAliases(args []string, stdout io.Writer) error {
	fs := newFlagSet("aliases")
	if err := parse(fs, args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return output.JSON(stdout, cfg.Aliases)
}

func runAliasSpace(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "usage: agent-kaiten-proxy alias-space <set|remove> [flags]", nil)
	}
	switch args[0] {
	case "set":
		fs := newFlagSet("alias-space set")
		alias := fs.String("alias", "", "alias name")
		spaceID := fs.String("space-id", "", "space id")
		hostName := fs.String("host-name", "", "configured host name")
		if err := parse(fs, args[1:]); err != nil {
			return err
		}
		if *alias == "" || *spaceID == "" {
			return apperr.New(apperr.CodeInvalidArgs, "--alias and --space-id are required", nil)
		}
		id, err := parsePositiveInt("space-id", *spaceID)
		if err != nil {
			return err
		}
		return saveAlias(stdout, func(cfg *config.Config) error {
			host, err := config.ResolveHost(*cfg, *hostName)
			if err != nil {
				return err
			}
			if err := config.ValidateAlias(*alias); err != nil {
				return err
			}
			cfg.Aliases.Spaces[*alias] = config.SpaceAlias{HostName: host.Name, SpaceID: id}
			return nil
		})
	case "remove":
		return removeAlias(args[1:], stdout, func(cfg *config.Config, alias string) { delete(cfg.Aliases.Spaces, alias) })
	default:
		return apperr.New(apperr.CodeInvalidArgs, "unknown alias-space subcommand", map[string]string{"subcommand": args[0]})
	}
}

func runAliasBoard(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "usage: agent-kaiten-proxy alias-board <set|remove> [flags]", nil)
	}
	switch args[0] {
	case "set":
		fs := newFlagSet("alias-board set")
		alias := fs.String("alias", "", "alias name")
		boardID := fs.String("board-id", "", "board id")
		spaceID := fs.String("space-id", "", "space id")
		hostName := fs.String("host-name", "", "configured host name")
		if err := parse(fs, args[1:]); err != nil {
			return err
		}
		if *alias == "" || *boardID == "" {
			return apperr.New(apperr.CodeInvalidArgs, "--alias and --board-id are required", nil)
		}
		bid, err := parsePositiveInt("board-id", *boardID)
		if err != nil {
			return err
		}
		sid := 0
		if *spaceID != "" {
			sid, err = parsePositiveInt("space-id", *spaceID)
			if err != nil {
				return err
			}
		}
		return saveAlias(stdout, func(cfg *config.Config) error {
			host, err := config.ResolveHost(*cfg, *hostName)
			if err != nil {
				return err
			}
			if err := config.ValidateAlias(*alias); err != nil {
				return err
			}
			cfg.Aliases.Boards[*alias] = config.BoardAlias{HostName: host.Name, SpaceID: sid, BoardID: bid}
			return nil
		})
	case "remove":
		return removeAlias(args[1:], stdout, func(cfg *config.Config, alias string) { delete(cfg.Aliases.Boards, alias) })
	default:
		return apperr.New(apperr.CodeInvalidArgs, "unknown alias-board subcommand", map[string]string{"subcommand": args[0]})
	}
}

func runAliasLane(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "usage: agent-kaiten-proxy alias-lane <set|remove> [flags]", nil)
	}
	switch args[0] {
	case "set":
		fs := newFlagSet("alias-lane set")
		alias := fs.String("alias", "", "alias name")
		laneID := fs.String("lane-id", "", "lane id")
		boardID := fs.String("board-id", "", "board id")
		hostName := fs.String("host-name", "", "configured host name")
		if err := parse(fs, args[1:]); err != nil {
			return err
		}
		if *alias == "" || *laneID == "" || *boardID == "" {
			return apperr.New(apperr.CodeInvalidArgs, "--alias, --lane-id and --board-id are required", nil)
		}
		lid, err := parsePositiveInt("lane-id", *laneID)
		if err != nil {
			return err
		}
		bid, err := parsePositiveInt("board-id", *boardID)
		if err != nil {
			return err
		}
		return saveAlias(stdout, func(cfg *config.Config) error {
			host, err := config.ResolveHost(*cfg, *hostName)
			if err != nil {
				return err
			}
			if err := config.ValidateAlias(*alias); err != nil {
				return err
			}
			cfg.Aliases.Lanes[*alias] = config.LaneAlias{HostName: host.Name, BoardID: bid, LaneID: lid}
			return nil
		})
	case "remove":
		return removeAlias(args[1:], stdout, func(cfg *config.Config, alias string) { delete(cfg.Aliases.Lanes, alias) })
	default:
		return apperr.New(apperr.CodeInvalidArgs, "unknown alias-lane subcommand", map[string]string{"subcommand": args[0]})
	}
}

func runMyCards(args []string, stdout io.Writer) error {
	fs := newFlagSet("my-cards")
	hostName := fs.String("host-name", "", "configured host name")
	states := fs.String("states", "", "state ids, comma separated")
	if err := parse(fs, args); err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	user, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	userID := strconv.Itoa(user.ID)
	memberCards, err := client.ListCards(ctx, kaiten.CardFilters{MemberIDs: userID, States: *states, Limit: 100})
	if err != nil {
		return err
	}
	responsibleCards, err := client.ListCards(ctx, kaiten.CardFilters{ResponsibleIDs: userID, States: *states, Limit: 100})
	if err != nil {
		return err
	}
	cards, err := dedupeCards(append(memberCards, responsibleCards...))
	if err != nil {
		return err
	}
	return output.JSON(stdout, cards)
}

func runCards(args []string, stdout io.Writer) error {
	fs := newCardFilterFlags("cards")
	if err := parse(fs.fs, args); err != nil {
		return err
	}
	return listCards(stdout, fs, false)
}

func runSpaceCards(args []string, stdout io.Writer) error {
	fs := newCardFilterFlags("space-cards")
	details := fs.fs.Bool("details", false, "fetch each card detail")
	if err := parse(fs.fs, args); err != nil {
		return err
	}
	if *fs.spaceAlias == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--space is required", nil)
	}
	return listCards(stdout, fs, *details)
}

func runLaneCards(args []string, stdout io.Writer) error {
	fs := newCardFilterFlags("lane-cards")
	details := fs.fs.Bool("details", false, "fetch each card detail")
	if err := parse(fs.fs, args); err != nil {
		return err
	}
	if *fs.laneAlias == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--lane is required", nil)
	}
	return listCards(stdout, fs, *details)
}

func runCard(args []string, stdout io.Writer) error {
	fs := newFlagSet("card")
	idText := fs.String("id", "", "card id")
	hostName := fs.String("host-name", "", "configured host name")
	includeComments := fs.Bool("include-comments", false, "include card comments")
	includeHistory := fs.Bool("include-history", false, "include card history and baselines")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	card, err := client.GetCard(ctx, id)
	if err != nil {
		return err
	}
	if !*includeComments && !*includeHistory {
		return output.JSON(stdout, card)
	}
	out := map[string]any{"card": card}
	if *includeComments {
		comments, err := client.CardComments(ctx, id)
		if err != nil {
			return err
		}
		out["comments"] = comments
	}
	if *includeHistory {
		history, err := fetchCardHistory(client, ctx, id)
		if err != nil {
			return err
		}
		out["history"] = history
	}
	return output.JSON(stdout, out)
}

func runCardComments(args []string, stdout io.Writer) error {
	fs := newFlagSet("card-comments")
	idText := fs.String("id", "", "card id")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	comments, err := client.CardComments(ctx, id)
	if err != nil {
		return err
	}
	return output.JSON(stdout, comments)
}

func runCardHistory(args []string, stdout io.Writer) error {
	fs := newFlagSet("card-history")
	idText := fs.String("id", "", "card id")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	history, err := fetchCardHistory(client, ctx, id)
	if err != nil {
		return err
	}
	return output.JSON(stdout, history)
}

func fetchCardHistory(client *kaiten.Client, ctx context.Context, id int) (map[string]any, error) {
	locationHistory, err := client.CardLocationHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	baselines, err := client.CardBaselines(ctx, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"location_history": locationHistory,
		"baselines":        baselines,
	}, nil
}

func runSpaces(args []string, stdout io.Writer) error {
	fs := newFlagSet("spaces")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	spaces, err := client.ListSpaces(ctx)
	if err != nil {
		return err
	}
	return output.JSON(stdout, spaces)
}

func runBoards(args []string, stdout io.Writer) error {
	fs := newFlagSet("boards")
	hostName := fs.String("host-name", "", "configured host name")
	spaceIDText := fs.String("space-id", "", "space id")
	spaceAlias := fs.String("space", "", "space alias")
	if err := parse(fs, args); err != nil {
		return err
	}
	target, cfg, err := resolveTarget(*hostName, *spaceIDText, *spaceAlias, "", "", "", "")
	if err != nil {
		return err
	}
	if target.SpaceID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "--space-id or --space is required", nil)
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	boards, err := client.ListBoards(ctx, target.SpaceID)
	if err != nil {
		return err
	}
	return output.JSON(stdout, boards)
}

func runLanes(args []string, stdout io.Writer) error {
	fs := newFlagSet("lanes")
	hostName := fs.String("host-name", "", "configured host name")
	boardIDText := fs.String("board-id", "", "board id")
	boardAlias := fs.String("board", "", "board alias")
	if err := parse(fs, args); err != nil {
		return err
	}
	target, cfg, err := resolveTarget(*hostName, "", "", *boardIDText, *boardAlias, "", "")
	if err != nil {
		return err
	}
	if target.BoardID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "--board-id or --board is required", nil)
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lanes, err := client.ListLanes(ctx, target.BoardID)
	if err != nil {
		return err
	}
	return output.JSON(stdout, lanes)
}

func runColumns(args []string, stdout io.Writer) error {
	fs := newFlagSet("columns")
	hostName := fs.String("host-name", "", "configured host name")
	boardIDText := fs.String("board-id", "", "board id")
	boardAlias := fs.String("board", "", "board alias")
	if err := parse(fs, args); err != nil {
		return err
	}
	target, cfg, err := resolveTarget(*hostName, "", "", *boardIDText, *boardAlias, "", "")
	if err != nil {
		return err
	}
	if target.BoardID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "--board-id or --board is required", nil)
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	columns, err := client.ListColumns(ctx, target.BoardID)
	if err != nil {
		return err
	}
	return output.JSON(stdout, columns)
}

// checkLaneColumnNameConflicts rejects combining a name lookup with an id/alias for the same target.
func checkLaneColumnNameConflicts(laneIDText, laneAlias, laneName, columnIDText, columnName string) error {
	if laneName != "" && (laneIDText != "" || laneAlias != "") {
		return apperr.New(apperr.CodeInvalidArgs, "use only one of --lane-id, --lane or --lane-name", nil)
	}
	if columnName != "" && columnIDText != "" {
		return apperr.New(apperr.CodeInvalidArgs, "use only one of --column-id or --column-name", nil)
	}
	return nil
}

// resolveLaneColumnNames looks up lane and column ids by title on the given board.
// It only fetches from Kaiten for the names that were actually provided.
func resolveLaneColumnNames(ctx context.Context, client *kaiten.Client, boardID int, laneName, columnName string, laneID, columnID *int) error {
	if laneName == "" && columnName == "" {
		return nil
	}
	if boardID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "board is required to resolve --lane-name/--column-name", nil)
	}
	if laneName != "" {
		lanes, err := client.ListLanes(ctx, boardID)
		if err != nil {
			return err
		}
		id, err := matchNamedEntity(lanes, "lane", laneName)
		if err != nil {
			return err
		}
		*laneID = id
	}
	if columnName != "" {
		columns, err := client.ListColumns(ctx, boardID)
		if err != nil {
			return err
		}
		id, err := matchNamedEntity(flattenColumns(columns), "column", columnName)
		if err != nil {
			return err
		}
		*columnID = id
	}
	return nil
}

// flattenColumns expands parent columns so a named subcolumn can also be matched.
func flattenColumns(columns []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(columns))
	for _, raw := range columns {
		out = append(out, raw)
		var parent struct {
			Subcolumns []json.RawMessage `json:"subcolumns"`
		}
		if err := json.Unmarshal(raw, &parent); err == nil {
			out = append(out, parent.Subcolumns...)
		}
	}
	return out
}

// matchNamedEntity resolves a single id from a list of {id,title} objects by title.
// Exact (case-insensitive) match wins; otherwise a unique substring match is used.
func matchNamedEntity(items []json.RawMessage, kind, query string) (int, error) {
	want := strings.ToLower(strings.TrimSpace(query))
	type entity struct {
		id    int
		title string
	}
	var all, exact, partial []entity
	for _, raw := range items {
		var e struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return 0, apperr.Wrap(apperr.CodeKaitenAPI, "decode "+kind, err, nil)
		}
		if e.ID == 0 {
			continue
		}
		ent := entity{id: e.ID, title: e.Title}
		all = append(all, ent)
		switch title := strings.ToLower(strings.TrimSpace(e.Title)); {
		case title == want:
			exact = append(exact, ent)
		case want != "" && strings.Contains(title, want):
			partial = append(partial, ent)
		}
	}
	available := make([]string, 0, len(all))
	for _, e := range all {
		available = append(available, e.title)
	}
	picked := exact
	if len(picked) == 0 {
		picked = partial
	}
	if len(picked) == 0 {
		return 0, apperr.New(apperr.CodeNotFound, kind+" not found by name", map[string]any{"query": query, "available": available})
	}
	if len(picked) > 1 {
		matched := make([]string, 0, len(picked))
		for _, e := range picked {
			matched = append(matched, e.title)
		}
		return 0, apperr.New(apperr.CodeInvalidArgs, kind+" name is ambiguous", map[string]any{"query": query, "matched": matched, "available": available})
	}
	return picked[0].id, nil
}

// cardBoardID reads the board id of an existing card so name lookups work with just --id.
func cardBoardID(ctx context.Context, client *kaiten.Client, id int) (int, error) {
	card, err := client.GetCard(ctx, id)
	if err != nil {
		return 0, err
	}
	var data struct {
		BoardID int `json:"board_id"`
	}
	if err := json.Unmarshal(card, &data); err != nil {
		return 0, apperr.Wrap(apperr.CodeKaitenAPI, "decode card board id", err, nil)
	}
	if data.BoardID == 0 {
		return 0, apperr.New(apperr.CodeKaitenAPI, "card response has no board_id", nil)
	}
	return data.BoardID, nil
}

// stringList is a repeatable string flag (-flag a -flag b).
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func runUsers(args []string, stdout io.Writer) error {
	fs := newFlagSet("users")
	query := fs.String("query", "", "name, username or email substring")
	hostName := fs.String("host-name", "", "configured host name")
	limit := fs.Int("limit", 50, "max users to print")
	if err := parse(fs, args); err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users, err := client.ListUsers(ctx)
	if err != nil {
		return err
	}
	if q := strings.ToLower(strings.TrimSpace(*query)); q != "" {
		filtered := make([]json.RawMessage, 0, len(users))
		for _, raw := range users {
			if userMatchesQuery(raw, q) {
				filtered = append(filtered, raw)
			}
		}
		users = filtered
	}
	if *limit > 0 && len(users) > *limit {
		users = users[:*limit]
	}
	return output.JSON(stdout, users)
}

func runCardMembers(args []string, stdout io.Writer) error {
	fs := newFlagSet("card-members")
	idText := fs.String("id", "", "card id")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	members, err := client.CardMembers(ctx, id)
	if err != nil {
		return err
	}
	return output.JSON(stdout, members)
}

func runAddMember(args []string, stdout io.Writer) error {
	fs := newFlagSet("add-member")
	idText := fs.String("id", "", "card id")
	userIDText := fs.String("user-id", "", "user id")
	userName := fs.String("user-name", "", "user name/email to resolve")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	userID, err := resolveMemberArg(ctx, client, *userIDText, *userName)
	if err != nil {
		return err
	}
	if _, err := client.AddCardMember(ctx, id, userID); err != nil {
		return err
	}
	members, err := client.CardMembers(ctx, id)
	if err != nil {
		return err
	}
	return output.JSON(stdout, map[string]any{"status": "ok", "card_id": id, "user_id": userID, "members": members})
}

func runRemoveMember(args []string, stdout io.Writer) error {
	fs := newFlagSet("remove-member")
	idText := fs.String("id", "", "card id")
	userIDText := fs.String("user-id", "", "user id")
	userName := fs.String("user-name", "", "user name/email to resolve")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	userID, err := resolveMemberArg(ctx, client, *userIDText, *userName)
	if err != nil {
		return err
	}
	if err := client.RemoveCardMember(ctx, id, userID); err != nil {
		return err
	}
	members, err := client.CardMembers(ctx, id)
	if err != nil {
		return err
	}
	return output.JSON(stdout, map[string]any{"status": "ok", "card_id": id, "user_id": userID, "members": members})
}

func resolveMemberArg(ctx context.Context, client *kaiten.Client, userIDText, userName string) (int, error) {
	if userIDText != "" && strings.TrimSpace(userName) != "" {
		return 0, apperr.New(apperr.CodeInvalidArgs, "use only one of --user-id or --user-name", nil)
	}
	if userIDText != "" {
		return parsePositiveInt("user-id", userIDText)
	}
	if strings.TrimSpace(userName) == "" {
		return 0, apperr.New(apperr.CodeInvalidArgs, "--user-id or --user-name is required", nil)
	}
	return resolveUserID(ctx, client, userName)
}

// checkUserNameConflicts rejects combining an id flag with the matching name flag.
func checkUserNameConflicts(responsibleIDText, responsibleName, ownerIDText, ownerName string) error {
	if responsibleIDText != "" && strings.TrimSpace(responsibleName) != "" {
		return apperr.New(apperr.CodeInvalidArgs, "use only one of --responsible-id or --responsible-name", nil)
	}
	if ownerIDText != "" && strings.TrimSpace(ownerName) != "" {
		return apperr.New(apperr.CodeInvalidArgs, "use only one of --owner-id or --owner-name", nil)
	}
	return nil
}

func resolveUserNameFlags(ctx context.Context, client *kaiten.Client, input map[string]any, responsibleName, ownerName string) error {
	if strings.TrimSpace(responsibleName) != "" {
		id, err := resolveUserID(ctx, client, responsibleName)
		if err != nil {
			return err
		}
		input["responsible_id"] = id
	}
	if strings.TrimSpace(ownerName) != "" {
		id, err := resolveUserID(ctx, client, ownerName)
		if err != nil {
			return err
		}
		input["owner_id"] = id
	}
	return nil
}

// collectMemberIDs turns --member-id and --member-name flags into a unique, ordered id list.
func collectMemberIDs(ctx context.Context, client *kaiten.Client, memberIDs, memberNames stringList) ([]int, error) {
	seen := map[int]bool{}
	out := make([]int, 0, len(memberIDs)+len(memberNames))
	for _, s := range memberIDs {
		id, err := parsePositiveInt("member-id", s)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, q := range memberNames {
		id, err := resolveUserID(ctx, client, q)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

func addCardMembers(ctx context.Context, client *kaiten.Client, cardID int, memberIDs []int) error {
	for _, uid := range memberIDs {
		if _, err := client.AddCardMember(ctx, cardID, uid); err != nil {
			return err
		}
	}
	return nil
}

type kaitenUser struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func userMatchesQuery(raw json.RawMessage, lowerQuery string) bool {
	var u kaitenUser
	if err := json.Unmarshal(raw, &u); err != nil {
		return false
	}
	for _, field := range []string{u.FullName, u.Username, u.Email} {
		if strings.Contains(strings.ToLower(field), lowerQuery) {
			return true
		}
	}
	return false
}

// resolveUserID finds exactly one user id by email/name/username.
// Exact email wins, then exact full name or username, then a unique substring match.
func resolveUserID(ctx context.Context, client *kaiten.Client, query string) (int, error) {
	want := strings.ToLower(strings.TrimSpace(query))
	if want == "" {
		return 0, apperr.New(apperr.CodeInvalidArgs, "user query is empty", nil)
	}
	users, err := client.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	var exactEmail, exactName, partial []kaitenUser
	for _, raw := range users {
		var u kaitenUser
		if err := json.Unmarshal(raw, &u); err != nil || u.ID == 0 {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(u.Email))
		name := strings.ToLower(strings.TrimSpace(u.FullName))
		username := strings.ToLower(strings.TrimSpace(u.Username))
		switch {
		case email == want:
			exactEmail = append(exactEmail, u)
		case name == want || username == want:
			exactName = append(exactName, u)
		case strings.Contains(name, want) || strings.Contains(username, want) || strings.Contains(email, want):
			partial = append(partial, u)
		}
	}
	picked := exactEmail
	if len(picked) == 0 {
		picked = exactName
	}
	if len(picked) == 0 {
		picked = partial
	}
	if len(picked) == 0 {
		return 0, apperr.New(apperr.CodeNotFound, "user not found", map[string]any{"query": query})
	}
	if len(picked) > 1 {
		return 0, apperr.New(apperr.CodeInvalidArgs, "user query is ambiguous", map[string]any{"query": query, "matched": userSummaries(picked)})
	}
	return picked[0].ID, nil
}

func userSummaries(users []kaitenUser) []map[string]any {
	const max = 20
	out := make([]map[string]any, 0, len(users))
	for i, u := range users {
		if i >= max {
			break
		}
		out = append(out, map[string]any{"id": u.ID, "full_name": u.FullName, "email": u.Email})
	}
	return out
}

func runCreateCard(args []string, stdout io.Writer) error {
	fs := newFlagSet("create-card")
	hostName := fs.String("host-name", "", "configured host name")
	boardIDText := fs.String("board-id", "", "board id")
	boardAlias := fs.String("board", "", "board alias")
	laneIDText := fs.String("lane-id", "", "lane id")
	laneAlias := fs.String("lane", "", "lane alias")
	laneName := fs.String("lane-name", "", "lane title to resolve on the board")
	columnIDText := fs.String("column-id", "", "column id")
	columnName := fs.String("column-name", "", "column title to resolve on the board")
	title := fs.String("title", "", "card title")
	description := fs.String("description", "", "card description")
	responsibleIDText := fs.String("responsible-id", "", "responsible user id")
	responsibleName := fs.String("responsible-name", "", "responsible user name/email to resolve")
	ownerIDText := fs.String("owner-id", "", "owner user id")
	ownerName := fs.String("owner-name", "", "owner user name/email to resolve")
	var memberIDs, memberNames stringList
	fs.Var(&memberIDs, "member-id", "member user id (repeatable)")
	fs.Var(&memberNames, "member-name", "member user name/email to resolve (repeatable)")
	position := fs.String("position", "", "first or last")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *title == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--title is required", nil)
	}
	if err := checkUserNameConflicts(*responsibleIDText, *responsibleName, *ownerIDText, *ownerName); err != nil {
		return err
	}
	target, cfg, err := resolveTarget(*hostName, "", "", *boardIDText, *boardAlias, *laneIDText, *laneAlias)
	if err != nil {
		return err
	}
	if err := checkLaneColumnNameConflicts(*laneIDText, *laneAlias, *laneName, *columnIDText, *columnName); err != nil {
		return err
	}
	if target.BoardID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "--board-id/--board is required", nil)
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	columnID := 0
	if err := resolveLaneColumnNames(ctx, client, target.BoardID, *laneName, *columnName, &target.LaneID, &columnID); err != nil {
		return err
	}
	if target.LaneID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "--lane-id/--lane/--lane-name is required", nil)
	}
	input := map[string]any{"title": *title, "board_id": target.BoardID, "lane_id": target.LaneID}
	if columnID > 0 {
		input["column_id"] = columnID
	}
	if err := addOptionalCardFields(input, *columnIDText, *description, *responsibleIDText, *ownerIDText, *position); err != nil {
		return err
	}
	if err := resolveUserNameFlags(ctx, client, input, *responsibleName, *ownerName); err != nil {
		return err
	}
	members, err := collectMemberIDs(ctx, client, memberIDs, memberNames)
	if err != nil {
		return err
	}
	card, err := client.CreateCard(ctx, input)
	if err != nil {
		return err
	}
	if len(members) > 0 {
		cardID, err := kaiten.CardID(card)
		if err != nil {
			return err
		}
		if err := addCardMembers(ctx, client, cardID, members); err != nil {
			return err
		}
		if fresh, err := client.GetCard(ctx, cardID); err == nil {
			card = fresh
		}
	}
	return output.JSON(stdout, card)
}

func runUpdateCard(args []string, stdout io.Writer) error {
	fs := newFlagSet("update-card")
	idText := fs.String("id", "", "card id")
	hostName := fs.String("host-name", "", "configured host name")
	boardIDText := fs.String("board-id", "", "board id")
	boardAlias := fs.String("board", "", "board alias")
	laneIDText := fs.String("lane-id", "", "lane id")
	laneAlias := fs.String("lane", "", "lane alias")
	laneName := fs.String("lane-name", "", "lane title to resolve on the board")
	columnIDText := fs.String("column-id", "", "column id")
	columnName := fs.String("column-name", "", "column title to resolve on the board")
	title := fs.String("title", "", "card title")
	description := fs.String("description", "", "card description")
	responsibleIDText := fs.String("responsible-id", "", "responsible user id")
	responsibleName := fs.String("responsible-name", "", "responsible user name/email to resolve")
	ownerIDText := fs.String("owner-id", "", "owner user id")
	ownerName := fs.String("owner-name", "", "owner user name/email to resolve")
	var memberIDs, memberNames stringList
	fs.Var(&memberIDs, "member-id", "member user id to add (repeatable)")
	fs.Var(&memberNames, "member-name", "member user name/email to add (repeatable)")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	if err := checkUserNameConflicts(*responsibleIDText, *responsibleName, *ownerIDText, *ownerName); err != nil {
		return err
	}
	target, cfg, err := resolveTarget(*hostName, "", "", *boardIDText, *boardAlias, *laneIDText, *laneAlias)
	if err != nil {
		return err
	}
	if err := checkLaneColumnNameConflicts(*laneIDText, *laneAlias, *laneName, *columnIDText, *columnName); err != nil {
		return err
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	columnID := 0
	if *laneName != "" || *columnName != "" {
		boardID := target.BoardID
		if boardID == 0 {
			boardID, err = cardBoardID(ctx, client, id)
			if err != nil {
				return err
			}
		}
		if err := resolveLaneColumnNames(ctx, client, boardID, *laneName, *columnName, &target.LaneID, &columnID); err != nil {
			return err
		}
	}
	input := map[string]any{}
	if *title != "" {
		input["title"] = *title
	}
	if *description != "" {
		input["description"] = *description
	}
	if target.BoardID > 0 {
		input["board_id"] = target.BoardID
	}
	if target.LaneID > 0 {
		input["lane_id"] = target.LaneID
	}
	if columnID > 0 {
		input["column_id"] = columnID
	}
	if err := addOptionalCardFields(input, *columnIDText, "", *responsibleIDText, *ownerIDText, ""); err != nil {
		return err
	}
	if err := resolveUserNameFlags(ctx, client, input, *responsibleName, *ownerName); err != nil {
		return err
	}
	members, err := collectMemberIDs(ctx, client, memberIDs, memberNames)
	if err != nil {
		return err
	}
	if len(input) == 0 && len(members) == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "at least one editable field is required", nil)
	}
	var card json.RawMessage
	if len(input) > 0 {
		card, err = client.UpdateCard(ctx, id, input)
		if err != nil {
			return err
		}
	}
	if len(members) > 0 {
		if err := addCardMembers(ctx, client, id, members); err != nil {
			return err
		}
	}
	if card == nil || len(members) > 0 {
		if fresh, err := client.GetCard(ctx, id); err == nil {
			card = fresh
		}
	}
	return output.JSON(stdout, card)
}

func runCommentCard(args []string, stdout io.Writer) error {
	fs := newFlagSet("comment-card")
	idText := fs.String("id", "", "card id")
	text := fs.String("text", "", "comment text")
	filePath := fs.String("file", "", "path to a local text file to upload and attach to the comment")
	fileIDText := fs.String("file-id", "", "id of a file already attached to the card to reference in the comment")
	name := fs.String("name", "", "attachment name for --file (defaults to the file base name)")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	hasFile := strings.TrimSpace(*filePath) != "" || *fileIDText != ""
	if strings.TrimSpace(*text) == "" && !hasFile {
		return apperr.New(apperr.CodeInvalidArgs, "--text is required (or attach a file with --file/--file-id)", nil)
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var files []json.RawMessage
	if *fileIDText != "" {
		existing, err := client.CardFiles(ctx, id)
		if err != nil {
			return err
		}
		file, err := selectCardFile(existing, *fileIDText, "")
		if err != nil {
			return err
		}
		files = append(files, file)
	}
	if strings.TrimSpace(*filePath) != "" {
		content, attachName, err := readTextFileForUpload(*filePath, *name)
		if err != nil {
			return err
		}
		file, err := client.AttachFile(ctx, id, attachName, content)
		if err != nil {
			return err
		}
		files = append(files, file)
	}

	input := map[string]any{"text": *text}
	if len(files) > 0 {
		input["files"] = files
	}
	comment, err := client.CreateComment(ctx, id, input)
	if err != nil {
		return err
	}
	return output.JSON(stdout, comment)
}

// maxTextFileBytes caps both uploads and downloads handled as text.
const maxTextFileBytes = 5 << 20

func runCardFiles(args []string, stdout io.Writer) error {
	fs := newFlagSet("card-files")
	idText := fs.String("id", "", "card id")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	files, err := client.CardFiles(ctx, id)
	if err != nil {
		return err
	}
	return output.JSON(stdout, files)
}

func runAttachFile(args []string, stdout io.Writer) error {
	fs := newFlagSet("attach-file")
	idText := fs.String("id", "", "card id")
	filePath := fs.String("file", "", "path to a local text file")
	name := fs.String("name", "", "attachment name (defaults to the file base name)")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*filePath) == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--file is required", nil)
	}
	content, attachName, err := readTextFileForUpload(*filePath, *name)
	if err != nil {
		return err
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	file, err := client.AttachFile(ctx, id, attachName, content)
	if err != nil {
		return err
	}
	return output.JSON(stdout, file)
}

func runReadFile(args []string, stdout io.Writer) error {
	fs := newFlagSet("read-file")
	idText := fs.String("id", "", "card id")
	fileIDText := fs.String("file-id", "", "attached file id")
	name := fs.String("name", "", "attached file name to match")
	maxBytes := fs.Int("max-bytes", maxTextFileBytes, "reject files larger than this")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	if *fileIDText == "" && strings.TrimSpace(*name) == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--file-id or --name is required", nil)
	}
	if *fileIDText != "" && strings.TrimSpace(*name) != "" {
		return apperr.New(apperr.CodeInvalidArgs, "use only one of --file-id or --name", nil)
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	files, err := client.CardFiles(ctx, id)
	if err != nil {
		return err
	}
	meta, err := selectCardFile(files, *fileIDText, *name)
	if err != nil {
		return err
	}
	var fileURL, fileName string
	var fileSize int
	{
		var f struct {
			URL  string `json:"url"`
			Name string `json:"name"`
			Size int    `json:"size"`
		}
		_ = json.Unmarshal(meta, &f)
		fileURL, fileName, fileSize = f.URL, f.Name, f.Size
	}
	if fileURL == "" {
		return apperr.New(apperr.CodeKaitenAPI, "file has no download url", map[string]string{"name": fileName})
	}
	if fileSize > 0 && fileSize > *maxBytes {
		return apperr.New(apperr.CodeInvalidArgs, "file is larger than --max-bytes", map[string]any{"name": fileName, "size": fileSize, "max_bytes": *maxBytes})
	}
	content, contentType, err := client.DownloadFile(ctx, fileURL)
	if err != nil {
		return err
	}
	if len(content) > *maxBytes {
		return apperr.New(apperr.CodeInvalidArgs, "file is larger than --max-bytes", map[string]any{"name": fileName, "size": len(content), "max_bytes": *maxBytes})
	}
	if !isTextual(content) {
		return apperr.New(apperr.CodeInvalidArgs, "file is not a text file", map[string]any{"name": fileName, "content_type": contentType})
	}
	return output.JSON(stdout, map[string]any{
		"file":     meta,
		"encoding": "utf-8",
		"bytes":    len(content),
		"content":  string(content),
	})
}

// selectCardFile finds one file object by id or by name (exact ci match, else unique substring).
func selectCardFile(files []json.RawMessage, fileIDText, name string) (json.RawMessage, error) {
	if fileIDText != "" {
		wantID, err := parsePositiveInt("file-id", fileIDText)
		if err != nil {
			return nil, err
		}
		for _, raw := range files {
			var f struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(raw, &f); err != nil {
				return nil, apperr.Wrap(apperr.CodeKaitenAPI, "decode file", err, nil)
			}
			if f.ID == wantID {
				return raw, nil
			}
		}
		return nil, apperr.New(apperr.CodeNotFound, "file id not found on card", map[string]any{"file_id": wantID})
	}
	want := strings.ToLower(strings.TrimSpace(name))
	var exact, partial []json.RawMessage
	var available []string
	for _, raw := range files {
		var f struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, apperr.Wrap(apperr.CodeKaitenAPI, "decode file", err, nil)
		}
		available = append(available, f.Name)
		switch fn := strings.ToLower(strings.TrimSpace(f.Name)); {
		case fn == want:
			exact = append(exact, raw)
		case want != "" && strings.Contains(fn, want):
			partial = append(partial, raw)
		}
	}
	picked := exact
	if len(picked) == 0 {
		picked = partial
	}
	if len(picked) == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "file name not found on card", map[string]any{"query": name, "available": available})
	}
	if len(picked) > 1 {
		return nil, apperr.New(apperr.CodeInvalidArgs, "file name is ambiguous", map[string]any{"query": name, "available": available})
	}
	return picked[0], nil
}

// readTextFileForUpload reads a local file, enforces the text-only limits, and
// returns its bytes plus the attachment name (nameOverride or the file base name).
func readTextFileForUpload(path, nameOverride string) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", apperr.Wrap(apperr.CodeInvalidArgs, "read file", err, map[string]string{"file": path})
	}
	if info.IsDir() {
		return nil, "", apperr.New(apperr.CodeInvalidArgs, "--file is a directory", map[string]string{"file": path})
	}
	if info.Size() > maxTextFileBytes {
		return nil, "", apperr.New(apperr.CodeInvalidArgs, "file is too large for a text attachment", map[string]any{"file": path, "size": info.Size(), "max_bytes": maxTextFileBytes})
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", apperr.Wrap(apperr.CodeInvalidArgs, "read file", err, map[string]string{"file": path})
	}
	if !isTextual(content) {
		return nil, "", apperr.New(apperr.CodeInvalidArgs, "only text files are supported for now", map[string]string{"file": path})
	}
	name := strings.TrimSpace(nameOverride)
	if name == "" {
		name = filepath.Base(path)
	}
	return content, name, nil
}

// isTextual reports whether content can be treated as a UTF-8 text file:
// no NUL bytes, valid UTF-8, and few non-printable control characters.
func isTextual(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	if bytesIndexZero(content) {
		return false
	}
	if !utf8.Valid(content) {
		return false
	}
	control := 0
	for _, b := range content {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' && b != '\f' && b != '\v' {
			control++
		}
	}
	return control*100 <= len(content) // <= 1% control bytes
}

func bytesIndexZero(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func runInstallSkill(args []string, stdout io.Writer, skills map[string]string) error {
	fs := newFlagSet("install-skill")
	targetDir := fs.String("target-dir", "", "explicit skills directory (overrides --codex/--claude)")
	codexOnly := fs.Bool("codex", false, "install only into ~/.codex/skills")
	claudeOnly := fs.Bool("claude", false, "install only into ~/.claude/skills")
	if err := parse(fs, args); err != nil {
		return err
	}
	bases, err := installSkillBases(*targetDir, *codexOnly, *claudeOnly)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	installed := make([]string, 0, len(names)*len(bases))
	for _, base := range bases {
		for _, name := range names {
			content := strings.TrimSpace(skills[name])
			if content == "" {
				continue
			}
			skillDir := filepath.Join(base, name)
			if err := os.MkdirAll(skillDir, 0o700); err != nil {
				return apperr.Wrap(apperr.CodeConfig, "create skill dir", err, map[string]string{"path": skillDir})
			}
			skillPath := filepath.Join(skillDir, "SKILL.md")
			if err := os.WriteFile(skillPath, []byte(content), 0o600); err != nil {
				return apperr.Wrap(apperr.CodeConfig, "write skill", err, map[string]string{"path": skillPath})
			}
			installed = append(installed, skillPath)
		}
	}
	if len(installed) == 0 {
		return apperr.New(apperr.CodeKaitenAPI, "embedded skills are empty", nil)
	}
	return output.JSON(stdout, map[string]any{
		"status": "ok",
		"paths":  installed,
		"recommendations": []string{
			"Run: agent-kaiten-proxy config",
			"If no host is configured, run: agent-kaiten-proxy bootstrap --interactive",
			"Configure useful aliases with alias-space, alias-board and alias-lane.",
			"Ask Codex or Claude to use the kaiten-proxy skill for Kaiten card analysis.",
			"Use kaiten-card-comments and kaiten-card-history skills to load comments or history for one card.",
			"Use the kaiten-card-edit skill to create cards on any board/lane/column or apply corrections to a card.",
		},
	})
}

// installSkillBases returns the skills directories to write to.
// An explicit --target-dir wins; otherwise both ~/.codex/skills and ~/.claude/skills
// are used unless narrowed with --codex or --claude.
func installSkillBases(targetDir string, codexOnly, claudeOnly bool) ([]string, error) {
	if targetDir != "" {
		if codexOnly || claudeOnly {
			return nil, apperr.New(apperr.CodeInvalidArgs, "--target-dir cannot be combined with --codex/--claude", nil)
		}
		return []string{targetDir}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeConfig, "resolve home dir", err, nil)
	}
	var bases []string
	if !claudeOnly {
		bases = append(bases, filepath.Join(home, ".codex", "skills"))
	}
	if !codexOnly {
		bases = append(bases, filepath.Join(home, ".claude", "skills"))
	}
	return bases, nil
}

type cardFilterFlags struct {
	fs                 *flag.FlagSet
	hostName           *string
	spaceID            *string
	spaceAlias         *string
	boardID            *string
	boardAlias         *string
	laneID             *string
	laneAlias          *string
	states             *string
	includeDescription *bool
	limit              *int
	offset             *int
}

type target struct {
	HostName string
	SpaceID  int
	BoardID  int
	LaneID   int
}

func newCardFilterFlags(name string) cardFilterFlags {
	fs := newFlagSet(name)
	return cardFilterFlags{
		fs:                 fs,
		hostName:           fs.String("host-name", "", "configured host name"),
		spaceID:            fs.String("space-id", "", "space id"),
		spaceAlias:         fs.String("space", "", "space alias"),
		boardID:            fs.String("board-id", "", "board id"),
		boardAlias:         fs.String("board", "", "board alias"),
		laneID:             fs.String("lane-id", "", "lane id"),
		laneAlias:          fs.String("lane", "", "lane alias"),
		states:             fs.String("states", "", "state ids, comma separated"),
		includeDescription: fs.Bool("include-description", false, "include card descriptions"),
		limit:              fs.Int("limit", 100, "max cards"),
		offset:             fs.Int("offset", 0, "offset"),
	}
}

func listCards(stdout io.Writer, fs cardFilterFlags, details bool) error {
	target, cfg, err := resolveTarget(*fs.hostName, *fs.spaceID, *fs.spaceAlias, *fs.boardID, *fs.boardAlias, *fs.laneID, *fs.laneAlias)
	if err != nil {
		return err
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cards, err := client.ListCards(ctx, kaiten.CardFilters{
		SpaceID:            target.SpaceID,
		BoardID:            target.BoardID,
		LaneID:             target.LaneID,
		States:             *fs.states,
		IncludeDescription: *fs.includeDescription,
		Limit:              *fs.limit,
		Offset:             *fs.offset,
	})
	if err != nil {
		return err
	}
	if details {
		detailed := make([]json.RawMessage, 0, len(cards))
		for _, card := range cards {
			id, err := kaiten.CardID(card)
			if err != nil {
				return err
			}
			full, err := client.GetCard(ctx, id)
			if err != nil {
				return err
			}
			detailed = append(detailed, full)
		}
		cards = detailed
	}
	return output.JSON(stdout, cards)
}

func resolveTarget(hostName, spaceIDText, spaceAlias, boardIDText, boardAlias, laneIDText, laneAlias string) (target, config.Config, error) {
	if spaceIDText != "" && spaceAlias != "" {
		return target{}, config.Config{}, apperr.New(apperr.CodeInvalidArgs, "use either --space-id or --space", nil)
	}
	if boardIDText != "" && boardAlias != "" {
		return target{}, config.Config{}, apperr.New(apperr.CodeInvalidArgs, "use either --board-id or --board", nil)
	}
	if laneIDText != "" && laneAlias != "" {
		return target{}, config.Config{}, apperr.New(apperr.CodeInvalidArgs, "use either --lane-id or --lane", nil)
	}
	cfg, err := loadConfig()
	if err != nil {
		return target{}, config.Config{}, err
	}
	t := target{HostName: hostName}
	if spaceIDText != "" {
		t.SpaceID, err = parsePositiveInt("space-id", spaceIDText)
		if err != nil {
			return target{}, config.Config{}, err
		}
	}
	if boardIDText != "" {
		t.BoardID, err = parsePositiveInt("board-id", boardIDText)
		if err != nil {
			return target{}, config.Config{}, err
		}
	}
	if laneIDText != "" {
		t.LaneID, err = parsePositiveInt("lane-id", laneIDText)
		if err != nil {
			return target{}, config.Config{}, err
		}
	}
	if spaceAlias != "" {
		alias, err := config.ResolveSpaceAlias(cfg, spaceAlias)
		if err != nil {
			return target{}, config.Config{}, err
		}
		if err := mergeAliasHost(&t, alias.HostName); err != nil {
			return target{}, config.Config{}, err
		}
		t.SpaceID = alias.SpaceID
	}
	if boardAlias != "" {
		alias, err := config.ResolveBoardAlias(cfg, boardAlias)
		if err != nil {
			return target{}, config.Config{}, err
		}
		if err := mergeAliasHost(&t, alias.HostName); err != nil {
			return target{}, config.Config{}, err
		}
		if alias.SpaceID > 0 {
			t.SpaceID = alias.SpaceID
		}
		t.BoardID = alias.BoardID
	}
	if laneAlias != "" {
		alias, err := config.ResolveLaneAlias(cfg, laneAlias)
		if err != nil {
			return target{}, config.Config{}, err
		}
		if err := mergeAliasHost(&t, alias.HostName); err != nil {
			return target{}, config.Config{}, err
		}
		t.BoardID = alias.BoardID
		t.LaneID = alias.LaneID
	}
	return t, cfg, nil
}

func mergeAliasHost(t *target, aliasHost string) error {
	if aliasHost == "" {
		return nil
	}
	if t.HostName != "" && t.HostName != aliasHost {
		return apperr.New(apperr.CodeInvalidArgs, "alias host conflicts with --host-name", map[string]string{"alias_host": aliasHost, "host_name": t.HostName})
	}
	t.HostName = aliasHost
	return nil
}

func addOptionalCardFields(input map[string]any, columnIDText, description, responsibleIDText, ownerIDText, position string) error {
	var err error
	if description != "" {
		input["description"] = description
	}
	if columnIDText != "" {
		input["column_id"], err = parsePositiveInt("column-id", columnIDText)
		if err != nil {
			return err
		}
	}
	if responsibleIDText != "" {
		input["responsible_id"], err = parsePositiveInt("responsible-id", responsibleIDText)
		if err != nil {
			return err
		}
	}
	if ownerIDText != "" {
		input["owner_id"], err = parsePositiveInt("owner-id", ownerIDText)
		if err != nil {
			return err
		}
	}
	if position != "" {
		switch position {
		case "first":
			input["position"] = 1
		case "last":
			input["position"] = 2
		default:
			return apperr.New(apperr.CodeInvalidArgs, "--position must be first or last", map[string]string{"position": position})
		}
	}
	return nil
}

func dedupeCards(cards []json.RawMessage) ([]json.RawMessage, error) {
	seen := map[int]struct{}{}
	out := make([]json.RawMessage, 0, len(cards))
	for _, card := range cards {
		id, err := kaiten.CardID(card)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, card)
	}
	return out, nil
}

func saveAlias(stdout io.Writer, mutate func(*config.Config) error) error {
	path, cfg, err := loadConfigWithPath()
	if err != nil {
		return err
	}
	if err := mutate(&cfg); err != nil {
		return err
	}
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	return output.JSON(stdout, map[string]any{"status": "ok", "aliases": cfg.Aliases})
}

func removeAlias(args []string, stdout io.Writer, mutate func(*config.Config, string)) error {
	fs := newFlagSet("alias remove")
	alias := fs.String("alias", "", "alias name")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *alias == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--alias is required", nil)
	}
	return saveAlias(stdout, func(cfg *config.Config) error {
		mutate(cfg, *alias)
		return nil
	})
}

func clientForHost(hostName string) (*kaiten.Client, config.Host, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, config.Host{}, err
	}
	return clientForResolvedHost(cfg, hostName)
}

func clientForResolvedHost(cfg config.Config, hostName string) (*kaiten.Client, config.Host, error) {
	host, err := config.ResolveHost(cfg, hostName)
	if err != nil {
		return nil, config.Host{}, err
	}
	return newKaitenClient(host.URL, host.Token), host, nil
}

func loadConfig() (config.Config, error) {
	_, cfg, err := loadConfigWithPath()
	return cfg, err
}

func loadConfigWithPath() (string, config.Config, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", config.Config{}, err
	}
	return path, cfg, nil
}

func parsePositiveInt(name, value string) (int, error) {
	if value == "" {
		return 0, apperr.New(apperr.CodeInvalidArgs, "--"+name+" is required", nil)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, apperr.New(apperr.CodeInvalidArgs, "--"+name+" must be a positive integer", map[string]string{name: value})
	}
	return parsed, nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return apperr.Wrap(apperr.CodeInvalidArgs, "parse flags", err, nil)
	}
	if fs.NArg() != 0 {
		return apperr.New(apperr.CodeInvalidArgs, "unexpected positional arguments", map[string][]string{"args": fs.Args()})
	}
	return nil
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func writeHelp(stdout io.Writer, topic string) error {
	if topic == "" {
		_, _ = fmt.Fprint(stdout, rootHelp)
		return nil
	}
	text, ok := commandHelp[topic]
	if !ok {
		return apperr.New(apperr.CodeInvalidArgs, "unknown help topic", map[string]string{"topic": topic})
	}
	_, _ = fmt.Fprint(stdout, text)
	return nil
}

const rootHelp = `agent-kaiten-proxy is a JSON-first Kaiten helper for AI agents.

Usage:
  agent-kaiten-proxy help [command]
  agent-kaiten-proxy <command> --help
  agent-kaiten-proxy <command> [flags]

Commands:
  bootstrap      Configure a Kaiten host and verify the token.
  config         Print configuration without tokens.
  set-default    Set default configured host.
  whoami         Print current Kaiten user.
  aliases        Print configured space, board and lane aliases.
  alias-space    Add or remove a space alias.
  alias-board    Add or remove a board alias.
  alias-lane     Add or remove a lane alias.
  my-cards       Print cards where current user is member or responsible.
  cards          Print filtered cards.
  space-cards    Print cards from a configured space alias.
  lane-cards     Print cards from a configured lane alias.
  card           Print one card, optionally with comments and history.
  card-comments  Print comments for a card.
  card-history   Print card movement history and baselines.
  spaces         Print spaces.
  boards         Print boards for a space.
  lanes          Print lanes for a board.
  columns        Print columns for a board.
  users          Print company users, optionally filtered by --query.
  create-card    Create a card on any board, lane and column.
  update-card    Update a card, including moving it between lanes and columns.
  comment-card   Add a card comment, optionally with a text file attached.
  card-files     List files attached to a card.
  attach-file    Attach a local text file to a card.
  read-file      Read the text content of a file attached to a card.
  card-members   List members of a card.
  add-member     Add a member to a card.
  remove-member  Remove a member from a card.
  install-skill  Install the embedded skills into Codex and Claude.
`

var commandHelp = map[string]string{
	"bootstrap": `Usage:
  agent-kaiten-proxy bootstrap --interactive
  agent-kaiten-proxy bootstrap --url <url> --token <token> --name <name> [--default]
`,
	"cards": `Usage:
  agent-kaiten-proxy cards [--space-id <id>|--space <alias>] [--board-id <id>|--board <alias>] [--lane-id <id>|--lane <alias>] [--states <csv>] [--include-description]
`,
	"card": `Usage:
  agent-kaiten-proxy card --id <card_id> [--include-comments] [--include-history]
`,
	"card-comments": `Usage:
  agent-kaiten-proxy card-comments --id <card_id>
`,
	"card-history": `Usage:
  agent-kaiten-proxy card-history --id <card_id>
`,
	"comment-card": `Usage:
  agent-kaiten-proxy comment-card --id <card_id> --text "<comment>"
  agent-kaiten-proxy comment-card --id <card_id> [--text "<comment>"] --file <path> [--name <attachment name>]
  agent-kaiten-proxy comment-card --id <card_id> [--text "<comment>"] --file-id <id>

--file uploads a local text file (same text-only limits as attach-file) and attaches it to the comment.
--file-id references a file already attached to the card. Both may be combined; --text is optional when a file is attached.
`,
	"card-files": `Usage:
  agent-kaiten-proxy card-files --id <card_id>
`,
	"attach-file": `Usage:
  agent-kaiten-proxy attach-file --id <card_id> --file <path> [--name <attachment name>]

Only text files are supported for now (valid UTF-8, no NUL bytes, up to 5 MiB).
`,
	"read-file": `Usage:
  agent-kaiten-proxy read-file --id <card_id> (--file-id <id> | --name <file name>) [--max-bytes <n>]

Downloads one attached file and prints {"file": <meta>, "content": <text>}.
Rejects non-text files. --name matches by exact (case-insensitive) name, else a unique substring.
`,
	"lanes": `Usage:
  agent-kaiten-proxy lanes [--board-id <id>|--board <alias>]
`,
	"columns": `Usage:
  agent-kaiten-proxy columns [--board-id <id>|--board <alias>]
`,
	"users": `Usage:
  agent-kaiten-proxy users [--query "<name|username|email>"] [--limit <n>]

--query is a case-insensitive substring matched against full_name, username and email.
`,
	"card-members": `Usage:
  agent-kaiten-proxy card-members --id <card_id>
`,
	"add-member": `Usage:
  agent-kaiten-proxy add-member --id <card_id> (--user-id <id> | --user-name "<name|email>")
`,
	"remove-member": `Usage:
  agent-kaiten-proxy remove-member --id <card_id> (--user-id <id> | --user-name "<name|email>")
`,
	"create-card": `Usage:
  agent-kaiten-proxy create-card (--board-id <id>|--board <alias>) (--lane-id <id>|--lane <alias>|--lane-name "<title>") --title "<title>" [--description "<text>"] [--column-id <id>|--column-name "<title>"] [--responsible-id <id>|--responsible-name "<name|email>"] [--owner-id <id>|--owner-name "<name|email>"] [--member-id <id> ...] [--member-name "<name|email>" ...] [--position first|last]

--lane-name and --column-name are matched against the board's lanes/columns by title
(exact case-insensitive match, otherwise a unique substring match).
--responsible-name/--owner-name/--member-name resolve a user by exact email, then exact
full name or username, then a unique substring. --member-id/--member-name are repeatable
and are added to the card after it is created.
`,
	"update-card": `Usage:
  agent-kaiten-proxy update-card --id <card_id> [--title "<title>"] [--description "<text>"] [--board-id <id>|--board <alias>] [--lane-id <id>|--lane <alias>|--lane-name "<title>"] [--column-id <id>|--column-name "<title>"] [--responsible-id <id>|--responsible-name "<name|email>"] [--owner-id <id>|--owner-name "<name|email>"] [--member-id <id> ...] [--member-name "<name|email>" ...]

--lane-name/--column-name resolve against the card's current board when --board-id/--board is omitted.
--member-id/--member-name are repeatable and are added to the card (they do not remove existing members;
use remove-member for that). A members-only update needs no other editable field.
`,
	"install-skill": `Usage:
  agent-kaiten-proxy install-skill [--target-dir <dir>] [--codex] [--claude]

By default the skills are written to both ~/.codex/skills and ~/.claude/skills.
Use --codex or --claude to install into only one of them, or --target-dir <dir>
to write to an explicit directory.
`,
}

type bootstrapInput struct {
	URL   string
	Token string
	Name  string
}

func promptBootstrap(stdin io.Reader, stdout io.Writer) (bootstrapInput, error) {
	reader := bufio.NewReader(stdin)
	urlValue, err := promptLine(reader, stdout, "Kaiten URL: ")
	if err != nil {
		return bootstrapInput{}, err
	}
	tokenValue, err := promptLine(reader, stdout, "Access token: ")
	if err != nil {
		return bootstrapInput{}, err
	}
	nameValue, err := promptLine(reader, stdout, "Host name: ")
	if err != nil {
		return bootstrapInput{}, err
	}
	return bootstrapInput{URL: urlValue, Token: tokenValue, Name: nameValue}, nil
}

func promptLine(reader *bufio.Reader, stdout io.Writer, prompt string) (string, error) {
	_, _ = fmt.Fprint(stdout, prompt)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", apperr.Wrap(apperr.CodeInvalidArgs, "read interactive input", err, nil)
	}
	return strings.TrimSpace(value), nil
}
