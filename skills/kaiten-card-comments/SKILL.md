---
name: kaiten-card-comments
description: Load all comments of a Kaiten card through the local agent-kaiten-proxy CLI. Use when you need to analyze the discussion on a card — decisions, remarks, clarifications of requirements, or how the ТЗ changed based on what people wrote in comments.
---

# Kaiten Card Comments

Fetch every comment of a single Kaiten card:

```bash
agent-kaiten-proxy card-comments --id <card_id>
```

Read-only command. Returns JSON on stdout and JSON errors on stderr.

## Output

An array of comment objects. Each comment has:

- `id` — comment id
- `text` — comment text (markdown)
- `created` — creation timestamp
- `updated` — last update timestamp
- `edited` — whether the comment was edited
- `author_id` / `author` — who wrote it (author object includes `full_name`, `username`, `email`)
- `card_id` — the card
- `internal` — internal flag
- `deleted` — deleted flag

## First Checks

If the host is not configured yet, check `agent-kaiten-proxy config`. If no host exists, ask the user for Kaiten host and token setup, then run:

```bash
agent-kaiten-proxy bootstrap --interactive
```

## Analysis Tips

- Sort comments by `created` to reconstruct the discussion timeline.
- Compare what is claimed or requested in comments with the current card description to detect requirement changes.
- Look for decisions (approvals, rejections, scope changes) and remarks (review feedback, blockers, open questions).
- The related skill `kaiten-proxy` can also pull the card and its history together with comments (`card --id <id> --include-comments --include-history`).
- Use the `kaiten-card-history` skill to load the card movement history separately.
