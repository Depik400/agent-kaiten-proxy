# agent-kaiten-proxy

`agent-kaiten-proxy` is a JSON-first Kaiten CLI helper for AI agents. It stores Kaiten hosts and tokens locally, remembers aliases for spaces, boards and lanes, reads cards for analysis, and can create, update or comment cards when requested.

## Install

```bash
go install github.com/Depik400/agent-kaiten-proxy@latest
```

From this repository:

```bash
go install -buildvcs=false .
```

## Configure

Interactive setup:

```bash
agent-kaiten-proxy bootstrap --interactive
```

Non-interactive setup:

```bash
agent-kaiten-proxy bootstrap \
  --url https://example.kaiten.ru \
  --token <token> \
  --name Main \
  --default
```

Config is stored in `~/.config/kaiten-proxy/config.json`. Override the path with `KAITEN_PROXY_CONFIG`.

Show config without tokens:

```bash
agent-kaiten-proxy config
```

## Aliases

```bash
agent-kaiten-proxy spaces
agent-kaiten-proxy boards --space-id 1
agent-kaiten-proxy lanes --board-id 10
agent-kaiten-proxy columns --board-id 10

agent-kaiten-proxy alias-space set --alias product --space-id 1
agent-kaiten-proxy alias-board set --alias main --space-id 1 --board-id 10
agent-kaiten-proxy alias-lane set --alias bugs --board-id 10 --lane-id 22
agent-kaiten-proxy aliases
```

## Cards

```bash
agent-kaiten-proxy my-cards
agent-kaiten-proxy cards --space product --include-description
agent-kaiten-proxy lane-cards --lane bugs --details --include-description
agent-kaiten-proxy card --id 123
```

Deep analysis of one card — pull the card together with its comments and history:

```bash
agent-kaiten-proxy card --id 123 --include-comments --include-history
```

The response contains `card` (current state), `comments` (discussion/decisions) and `history` (`location_history` — every move between lanes/columns, and `baselines` — planned date changes). Use it to understand how the ТЗ and requirements changed over time.

Comments or history alone:

```bash
agent-kaiten-proxy card-comments --id 123
agent-kaiten-proxy card-history --id 123
```

Write operations:

```bash
# Aliased board/lane
agent-kaiten-proxy create-card --board main --lane bugs --title "Bug title" --description "Details"

# Any board, with the lane and column resolved from plain text (matched by title)
agent-kaiten-proxy create-card --board-id 1040055 --lane-name "Павел Кононов" --column-name "TO DO" --title "Bug title"

# Raw ids
agent-kaiten-proxy create-card --board-id 1040055 --lane-id 22 --column-id 7 --title "Bug title"

# Edit a card / move it between lanes and columns (names resolve on the card's current board)
agent-kaiten-proxy update-card --id 123 --description "Updated details"
agent-kaiten-proxy update-card --id 123 --column-name "IN PROGRESS"
agent-kaiten-proxy update-card --id 123 --board-id 1040055 --lane-name "Павел Кононов" --column-name "TO DO"

agent-kaiten-proxy comment-card --id 123 --text "Investigated by agent"
```

`--lane-name` / `--column-name` match a lane or column title on the board (exact case-insensitive
match, otherwise a unique substring match; subcolumn titles are matched too). An ambiguous or
missing name returns an error listing the available titles.

### Files (text only)

Attach and read card files. Only text files are supported for now (valid UTF-8, no NUL bytes, up to 5 MiB).

```bash
agent-kaiten-proxy card-files --id 123
agent-kaiten-proxy attach-file --id 123 --file ./notes.md
agent-kaiten-proxy attach-file --id 123 --file ./log.txt --name run-log.txt
agent-kaiten-proxy read-file --id 123 --name notes.md
agent-kaiten-proxy read-file --id 123 --file-id 4567 --max-bytes 1048576
```

`read-file` prints `{"file": <meta>, "encoding": "utf-8", "bytes": <n>, "content": "<text>"}` and
rejects non-text files.

## Install the Skills

```bash
agent-kaiten-proxy install-skill            # both ~/.codex/skills/ and ~/.claude/skills/
agent-kaiten-proxy install-skill --claude   # only ~/.claude/skills/
agent-kaiten-proxy install-skill --codex    # only ~/.codex/skills/
agent-kaiten-proxy install-skill --target-dir ./skills-out
```

The command writes the embedded skills (each in its own directory) and prints recommendations for use:

- `kaiten-proxy` — full Kaiten workflow, including deep card analysis with comments and history.
- `kaiten-card-history` — loads the history of one card (`card-history`).
- `kaiten-card-comments` — loads the comments of one card (`card-comments`).
- `kaiten-card-edit` — creates cards on any board/lane/column and applies corrections to a card (`create-card`, `update-card`).

## Exit Codes

- `2`: invalid arguments
- `3`: config error
- `4`: auth error
- `5`: not found
- `7`: Kaiten API error
