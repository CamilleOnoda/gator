package app

import (
	"fmt"

	"github.com/CamilleOnoda/gator/internal/config"
	"github.com/CamilleOnoda/gator/internal/database"
)

type State struct {
	Db  *database.Queries
	Cfg *config.Config
}

type Command struct {
	Name string
	Args []string
}

type CLIcommands struct {
	Cmd map[string]func(*State, Command) error
}

func (c *CLIcommands) Run(s *State, cmd Command) error {
	if handler, exists := c.Cmd[cmd.Name]; exists {
		return handler(s, cmd)
	} else {
		return fmt.Errorf("Unknown command: %s", cmd.Name)
	}
}

func (c *CLIcommands) Register(name string, f func(*State, Command) error) {
	c.Cmd[name] = f
}
