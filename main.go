package main

import (
	_ "embed"
	"os"

	"github.com/Depik400/agent-kaiten-proxy/internal/cli"
)

//go:embed skills/kaiten-proxy/SKILL.md
var embeddedKaitenProxySkill string

//go:embed skills/kaiten-card-history/SKILL.md
var embeddedCardHistorySkill string

//go:embed skills/kaiten-card-comments/SKILL.md
var embeddedCardCommentsSkill string

//go:embed skills/kaiten-card-edit/SKILL.md
var embeddedCardEditSkill string

func main() {
	skills := map[string]string{
		"kaiten-proxy":         embeddedKaitenProxySkill,
		"kaiten-card-history":  embeddedCardHistorySkill,
		"kaiten-card-comments": embeddedCardCommentsSkill,
		"kaiten-card-edit":     embeddedCardEditSkill,
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, skills))
}
