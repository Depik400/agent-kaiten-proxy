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

Write operations:

```bash
agent-kaiten-proxy create-card --board main --lane bugs --title "Bug title" --description "Details"
agent-kaiten-proxy update-card --id 123 --description "Updated details"
agent-kaiten-proxy comment-card --id 123 --text "Investigated by agent"
```

## Install the Codex Skill

```bash
agent-kaiten-proxy install-skill
```

The command writes the embedded skill to `~/.codex/skills/kaiten-proxy/SKILL.md` and prints recommendations for use.

## Exit Codes

- `2`: invalid arguments
- `3`: config error
- `4`: auth error
- `5`: not found
- `7`: Kaiten API error
