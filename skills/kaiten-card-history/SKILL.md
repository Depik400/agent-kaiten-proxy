---
name: kaiten-card-history
description: Load the full history of a Kaiten card through the local agent-kaiten-proxy CLI — every movement between boards, columns and lanes plus baseline (planned date) changes. Use when you need to reconstruct how a card evolved over time, how the ТЗ and requirements changed, and who moved it where and when.
---

# Kaiten Card History

Fetch the full history of a single Kaiten card:

```bash
agent-kaiten-proxy card-history --id <card_id>
```

Read-only command. Returns JSON on stdout and JSON errors on stderr.

## Output

An object with two parts:

- `location_history` — array of movement events, each with:
  - `id` — movement event id
  - `board_id`, `column_id`, `subcolumn_id`, `lane_id` — where the card was moved to
  - `sprint_id` — sprint, when the card belongs to one
  - `author_id` / `author` — who performed the movement (author object includes `full_name`, `username`)
  - `changed` — timestamp of the movement
- `baselines` — array of baseline (planned start/end) records with `project_id`, `baseline_id`, `planned_start`, `planned_end`. Multiple records mean the plan changed over time.

## First Checks

If the host is not configured yet, check `agent-kaiten-proxy config`. If no host exists, ask the user for Kaiten host and token setup, then run:

```bash
agent-kaiten-proxy bootstrap --interactive
```

## Analysis Tips

- Sort `location_history` by `changed` to reconstruct the card lifecycle: where it was created, which lanes/columns it passed through, and when.
- Use `author` on each movement to see who drove the changes.
- Compare `baselines` entries to see how planned dates shifted.
- Combine history with comments (see the `kaiten-card-comments` skill) to understand why the card moved and how requirements evolved.
- The related skill `kaiten-proxy` can also pull the card, comments and history in one call (`card --id <id> --include-comments --include-history`).
