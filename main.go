package main

import (
	_ "embed"
	"os"

	"github.com/Depik400/agent-kaiten-proxy/internal/cli"
)

//go:embed skills/kaiten-proxy/SKILL.md
var embeddedSkill string

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, embeddedSkill))
}
