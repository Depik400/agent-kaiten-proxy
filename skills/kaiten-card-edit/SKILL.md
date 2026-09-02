---
name: kaiten-card-edit
description: Create and edit Kaiten cards through the local agent-kaiten-proxy CLI — put a card on any space, board, lane and column, apply corrections to an existing card (title, description, responsible, owner, or move it between lanes and columns), and attach or read text files on a card. Use whenever the user wants to add a task to Kaiten, fix an existing card, or work with a card's attachments.
---

# Kaiten Card Create & Edit

Use `agent-kaiten-proxy` as the only interface to Kaiten. It prints JSON on stdout and JSON errors on stderr.

Treat every command here as a **write operation**. Run it only after the user has clearly asked to create or change a card, and only after you have confirmed the resolved board, lane and column back to the user.

## First Checks

```bash
agent-kaiten-proxy config
```

If no host is configured, ask the user for the Kaiten host and token, then run:

```bash
agent-kaiten-proxy bootstrap --interactive
```

## Creating a card

### 1. Get the space and board (ask the user explicitly)

Ask the user for the target **space** and **board**. They may give names, ids, or a Kaiten board URL such as:

```
https://mpstats.kaiten.ru/space/449289/boards?focus=board&focusId=1040055
```

Parse the URL: the number after `/space/` is the **space id** (`449289`); `focusId` is the **board id** (`1040055`).

Confirm the board exists and note its id:

```bash
agent-kaiten-proxy boards --space-id <space_id>
```

Match the board the user named (e.g. "Core в разработке") to an `id` in the output.

### 2. Resolve the lane and column from plain text

The user names the lane and column in words — e.g. lane "Павел Кононов", column "TO DO". List them and match by title:

```bash
agent-kaiten-proxy lanes --board-id <board_id>
agent-kaiten-proxy columns --board-id <board_id>
```

You can either pass the resolved ids (`--lane-id`, `--column-id`), or let the CLI match the title for you with `--lane-name` / `--column-name` (exact case-insensitive match, otherwise a unique substring match; subcolumn titles are matched too). If a name is ambiguous or missing, the error lists the available titles — show them to the user and ask.

### 3. Create

```bash
agent-kaiten-proxy create-card \
  --board-id <board_id> \
  --lane-name "Павел Кононов" \
  --column-name "TO DO" \
  --title "<title>" \
  --description "<markdown description>"
```

Equivalent with ids:

```bash
agent-kaiten-proxy create-card --board-id <board_id> --lane-id <lane_id> --column-id <column_id> --title "<title>"
```

Optional flags: `--responsible-id <id>`, `--owner-id <id>`, `--position first|last`.
A board alias works in place of `--board-id`: `--board <alias>`. A saved lane alias works as `--lane <alias>`.

`--title` is required. `board` and a `lane` (id, alias, or name) are required. Column is optional — omit it to use the board's default first column.

The command prints the created card JSON. Report the new card id and its URL to the user.

## Editing a card

Apply only the fields the user asked to change:

```bash
agent-kaiten-proxy update-card --id <card_id> --description "<new description>"
agent-kaiten-proxy update-card --id <card_id> --title "<new title>"
agent-kaiten-proxy update-card --id <card_id> --responsible-id <id> --owner-id <id>
```

Move a card between lanes / columns. `--lane-name` and `--column-name` resolve against the card's **current board** when you do not pass `--board-id`/`--board`:

```bash
agent-kaiten-proxy update-card --id <card_id> --column-name "IN PROGRESS"
agent-kaiten-proxy update-card --id <card_id> --lane-name "Павел Кононов" --column-name "TO DO"
```

Move a card to another board (pass the board so the lane/column resolve there):

```bash
agent-kaiten-proxy update-card --id <card_id> --board-id <board_id> --lane-name "<lane>" --column-name "<column>"
```

At least one editable field is required. The command prints the updated card JSON.

## Files (text only)

Only text files are supported for now: valid UTF-8, no NUL bytes, up to 5 MiB. Binary files are rejected with an error.

List files on a card:

```bash
agent-kaiten-proxy card-files --id <card_id>
```

Each entry has `id`, `name`, `url`, `size`, `created`, author info.

Attach a local text file (write operation — confirm with the user first):

```bash
agent-kaiten-proxy attach-file --id <card_id> --file ./path/to/notes.md
agent-kaiten-proxy attach-file --id <card_id> --file ./log.txt --name "run-log.txt"
```

Read the text content of a file that is already attached to a card:

```bash
agent-kaiten-proxy read-file --id <card_id> --name notes.md
agent-kaiten-proxy read-file --id <card_id> --file-id 12345
```

`read-file` prints `{"file": <meta>, "encoding": "utf-8", "bytes": <n>, "content": "<text>"}`.
`--name` matches by exact (case-insensitive) name, otherwise a unique substring; an ambiguous or
missing name returns an error listing the available names. Use `card-files` first to see them.
A non-text file (or one larger than `--max-bytes`, default 5 MiB) is rejected — do not try to
work around it.

Attach a file to a comment (write operation — confirm first):

```bash
# upload a local text file and attach it to the comment
agent-kaiten-proxy comment-card --id <card_id> --text "<comment>" --file ./run.log

# reference a file that is already attached to the card
agent-kaiten-proxy comment-card --id <card_id> --text "<comment>" --file-id <file_id>
```

`--text` is optional when a file is attached. `--file` follows the same text-only limits as
`attach-file` (it uploads the file to the card, then links it in the comment). `--file` and
`--file-id` may be combined to attach several files.

## Rules

- Never guess space, board, lane or column ids. Resolve them from `boards`, `lanes`, `columns` output or from a URL the user gave.
- Always echo the resolved board / lane / column titles to the user and get a confirmation before running `create-card` or `update-card`.
- If a `--lane-name` / `--column-name` lookup fails, do not fall back to a guess — show the `available` titles from the error and ask the user which one.
- Only text files can be attached or read for now. State this plainly if the user asks for a binary file.
- `attach-file` is a write operation — confirm the target card and file with the user before running it.
- For reading cards, comments or history use the `kaiten-proxy`, `kaiten-card-comments` and `kaiten-card-history` skills.
