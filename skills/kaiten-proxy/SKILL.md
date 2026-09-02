---
name: kaiten-proxy
description: Work with Kaiten through the local agent-kaiten-proxy CLI. Use when Codex needs to inspect Kaiten spaces, boards, lanes, cards, aliases, current-user cards, analyze cards from a saved space or lane alias, create cards, update cards, or add card comments through Kaiten API.
---
# Kaiten Proxy

Use `agent-kaiten-proxy` as the only interface to Kaiten. It returns JSON on stdout and JSON errors on stderr.

## First Checks

Run:

```bash
agent-kaiten-proxy config
agent-kaiten-proxy aliases
```

If no host is configured, ask the user for Kaiten host and token setup, then run:

```bash
agent-kaiten-proxy bootstrap --interactive
```

## Analysis Workflow

For analysis of a space or lane, first ask the user for the alias of the target space or lane.

If the alias exists, use one of:

```bash
agent-kaiten-proxy space-cards --space <alias> --details --include-description
agent-kaiten-proxy lane-cards --lane <alias> --details --include-description
```

Analyze the returned card JSON, including title, description, owner, members, responsible user, board, lane, state, blockers, due dates, and comments counters when present.

For a deep analysis of one card, always pull the card together with its comments and history so you can understand how the requirements (ТЗ) changed over time and which decisions or remarks were made in the discussion:

```bash
agent-kaiten-proxy card --id <card_id> --include-comments --include-history
```

The response contains three parts:

- `card` — the full current card (title, description, owner, members, responsible user, state, lane, due dates, blockers).
- `comments` — chronological comments (author, created, text, `edited`, `internal`). Use them to find decisions, remarks, clarifications and changed requirements.
- `history` — card lifecycle: `location_history` (every movement between boards/columns/lanes with author and `changed` timestamp) and `baselines` (planned start/end changes over time).

Reconstruct the timeline from `history` and `comments`, compare comment claims with the current description, and report how the ТЗ and requirements evolved.

If the user does not have an alias, help configure one:

1. Find spaces:
   ```bash
   agent-kaiten-proxy spaces
   ```
2. For a board alias, find boards:
   ```bash
   agent-kaiten-proxy boards --space-id <space_id>
   ```
3. For a lane alias, find lanes:
   ```bash
   agent-kaiten-proxy lanes --board-id <board_id>
   ```
4. Save the alias:
   ```bash
   agent-kaiten-proxy alias-space set --alias <alias> --space-id <space_id>
   agent-kaiten-proxy alias-board set --alias <alias> --space-id <space_id> --board-id <board_id>
   agent-kaiten-proxy alias-lane set --alias <alias> --board-id <board_id> --lane-id <lane_id>
   ```

Do not guess Kaiten IDs for analysis. Use aliases or help the user create aliases.

## Common Commands

Current user's cards:

```bash
agent-kaiten-proxy my-cards
```

Filtered cards:

```bash
agent-kaiten-proxy cards --board <alias> --include-description
agent-kaiten-proxy cards --lane <alias> --states 1,2 --include-description
```

One card:

```bash
agent-kaiten-proxy card --id <card_id>
agent-kaiten-proxy card --id <card_id> --include-comments --include-history
```

Comments or history for one card:

```bash
agent-kaiten-proxy card-comments --id <card_id>
agent-kaiten-proxy card-history --id <card_id>
```

The dedicated `kaiten-card-comments` and `kaiten-card-history` skills cover these two commands separately.

Create a card:

```bash
agent-kaiten-proxy create-card --board <board_alias> --lane <lane_alias> --title "<title>" --description "<description>"
```

Create a card on any board without an alias — resolve the lane and column from plain text:

```bash
agent-kaiten-proxy lanes --board-id <board_id>
agent-kaiten-proxy columns --board-id <board_id>
agent-kaiten-proxy create-card --board-id <board_id> --lane-name "<lane title>" --column-name "<column title>" --title "<title>"
```

Update a card or comment only when the user explicitly asks to change Kaiten:

```bash
agent-kaiten-proxy update-card --id <card_id> --description "<description>"
agent-kaiten-proxy update-card --id <card_id> --column-name "<column title>"
agent-kaiten-proxy comment-card --id <card_id> --text "<comment>"
```

Attach or read text files on a card (text files only, up to 5 MiB):

```bash
agent-kaiten-proxy card-files --id <card_id>
agent-kaiten-proxy attach-file --id <card_id> --file ./notes.md
agent-kaiten-proxy read-file --id <card_id> --name notes.md
```

The dedicated `kaiten-card-edit` skill covers creating and editing cards in detail (parsing a board URL, resolving lane/column by title, moving cards, attaching and reading text files).

## Safety

Read-only commands are safe to run for context. Treat `create-card`, `update-card`, and `comment-card` as write operations and run them only after the user clearly requests that change.
