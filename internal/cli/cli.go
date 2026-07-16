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
	"strconv"
	"strings"
	"time"

	"github.com/Depik400/agent-kaiten-proxy/internal/apperr"
	"github.com/Depik400/agent-kaiten-proxy/internal/config"
	"github.com/Depik400/agent-kaiten-proxy/internal/kaiten"
	"github.com/Depik400/agent-kaiten-proxy/internal/output"
)

var newKaitenClient = kaiten.NewClient

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, embeddedSkill string) int {
	if len(args) == 0 {
		writeHelp(stdout, "")
		return apperr.ExitOK
	}
	if err := run(args, stdin, stdout, embeddedSkill); err != nil {
		output.Error(stderr, err)
		return apperr.ExitCode(err)
	}
	return apperr.ExitOK
}

func run(args []string, stdin io.Reader, stdout io.Writer, embeddedSkill string) error {
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
	case "spaces":
		return runSpaces(args[1:], stdout)
	case "boards":
		return runBoards(args[1:], stdout)
	case "lanes":
		return runLanes(args[1:], stdout)
	case "create-card":
		return runCreateCard(args[1:], stdout)
	case "update-card":
		return runUpdateCard(args[1:], stdout)
	case "comment-card":
		return runCommentCard(args[1:], stdout)
	case "install-skill":
		return runInstallSkill(args[1:], stdout, embeddedSkill)
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
	return output.JSON(stdout, card)
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

func runCreateCard(args []string, stdout io.Writer) error {
	fs := newFlagSet("create-card")
	hostName := fs.String("host-name", "", "configured host name")
	boardIDText := fs.String("board-id", "", "board id")
	boardAlias := fs.String("board", "", "board alias")
	laneIDText := fs.String("lane-id", "", "lane id")
	laneAlias := fs.String("lane", "", "lane alias")
	columnIDText := fs.String("column-id", "", "column id")
	title := fs.String("title", "", "card title")
	description := fs.String("description", "", "card description")
	responsibleIDText := fs.String("responsible-id", "", "responsible user id")
	ownerIDText := fs.String("owner-id", "", "owner user id")
	position := fs.String("position", "", "first or last")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *title == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--title is required", nil)
	}
	target, cfg, err := resolveTarget(*hostName, "", "", *boardIDText, *boardAlias, *laneIDText, *laneAlias)
	if err != nil {
		return err
	}
	if target.BoardID == 0 || target.LaneID == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "--board-id/--board and --lane-id/--lane are required", nil)
	}
	input := map[string]any{"title": *title, "board_id": target.BoardID, "lane_id": target.LaneID}
	if err := addOptionalCardFields(input, *columnIDText, *description, *responsibleIDText, *ownerIDText, *position); err != nil {
		return err
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	card, err := client.CreateCard(ctx, input)
	if err != nil {
		return err
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
	columnIDText := fs.String("column-id", "", "column id")
	title := fs.String("title", "", "card title")
	description := fs.String("description", "", "card description")
	responsibleIDText := fs.String("responsible-id", "", "responsible user id")
	ownerIDText := fs.String("owner-id", "", "owner user id")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	target, cfg, err := resolveTarget(*hostName, "", "", *boardIDText, *boardAlias, *laneIDText, *laneAlias)
	if err != nil {
		return err
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
	if err := addOptionalCardFields(input, *columnIDText, "", *responsibleIDText, *ownerIDText, ""); err != nil {
		return err
	}
	if len(input) == 0 {
		return apperr.New(apperr.CodeInvalidArgs, "at least one editable field is required", nil)
	}
	client, _, err := clientForResolvedHost(cfg, target.HostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	card, err := client.UpdateCard(ctx, id, input)
	if err != nil {
		return err
	}
	return output.JSON(stdout, card)
}

func runCommentCard(args []string, stdout io.Writer) error {
	fs := newFlagSet("comment-card")
	idText := fs.String("id", "", "card id")
	text := fs.String("text", "", "comment text")
	hostName := fs.String("host-name", "", "configured host name")
	if err := parse(fs, args); err != nil {
		return err
	}
	id, err := parsePositiveInt("id", *idText)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*text) == "" {
		return apperr.New(apperr.CodeInvalidArgs, "--text is required", nil)
	}
	client, _, err := clientForHost(*hostName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	comment, err := client.CommentCard(ctx, id, *text)
	if err != nil {
		return err
	}
	return output.JSON(stdout, comment)
}

func runInstallSkill(args []string, stdout io.Writer, embeddedSkill string) error {
	fs := newFlagSet("install-skill")
	targetDir := fs.String("target-dir", "", "Codex skills directory")
	if err := parse(fs, args); err != nil {
		return err
	}
	if strings.TrimSpace(embeddedSkill) == "" {
		return apperr.New(apperr.CodeKaitenAPI, "embedded skill is empty", nil)
	}
	base := *targetDir
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return apperr.Wrap(apperr.CodeConfig, "resolve home dir", err, nil)
		}
		base = filepath.Join(home, ".codex", "skills")
	}
	skillDir := filepath.Join(base, "kaiten-proxy")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		return apperr.Wrap(apperr.CodeConfig, "create skill dir", err, map[string]string{"path": skillDir})
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(embeddedSkill), 0o600); err != nil {
		return apperr.Wrap(apperr.CodeConfig, "write skill", err, map[string]string{"path": skillPath})
	}
	return output.JSON(stdout, map[string]any{
		"status": "ok",
		"path":   skillPath,
		"recommendations": []string{
			"Run: agent-kaiten-proxy config",
			"If no host is configured, run: agent-kaiten-proxy bootstrap --interactive",
			"Configure useful aliases with alias-space, alias-board and alias-lane.",
			"Ask Codex to use the kaiten-proxy skill for Kaiten card analysis.",
		},
	})
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
  card           Print one card.
  spaces         Print spaces.
  boards         Print boards for a space.
  lanes          Print lanes for a board.
  create-card    Create a card.
  update-card    Update a card.
  comment-card   Add a card comment.
  install-skill  Install the embedded Codex skill.
`

var commandHelp = map[string]string{
	"bootstrap": `Usage:
  agent-kaiten-proxy bootstrap --interactive
  agent-kaiten-proxy bootstrap --url <url> --token <token> --name <name> [--default]
`,
	"cards": `Usage:
  agent-kaiten-proxy cards [--space-id <id>|--space <alias>] [--board-id <id>|--board <alias>] [--lane-id <id>|--lane <alias>] [--states <csv>] [--include-description]
`,
	"install-skill": `Usage:
  agent-kaiten-proxy install-skill [--target-dir <dir>]
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
