package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/CamilleOnoda/blog-aggregator/internal/config"
	"github.com/CamilleOnoda/blog-aggregator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type CLIcommands struct {
	cmd map[string]func(*state, command) error
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Expected to receive a username after 'login' command")
	}

	username := cmd.args[0]
	if _, err := s.db.GetUser(context.Background(), username); err != nil {
		log.Fatal("User does not exist, register first")
	}
	err := config.SetUser(username)
	if err != nil {
		return fmt.Errorf("Error setting user: %v", err)
	}

	s.cfg.Current_user_name = username
	fmt.Println("User set to:", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("Expected to receive a username after 'register' command")
	}

	username := cmd.args[0]
	if _, err := s.db.GetUser(context.Background(), username); err == nil {
		log.Fatal("User already exists")
	}

	_, err := s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      username,
	})
	if err != nil {
		return fmt.Errorf("Error creating user: %v", err)
	}

	err = config.SetUser(username)
	if err != nil {
		return fmt.Errorf("Error setting user: %v", err)
	}

	s.cfg.Current_user_name = username
	fmt.Println("User registered and set to:", username)

	return nil
}

func handlerReset(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("Expected no arguments after 'reset' command")
	}
	if err := s.db.Reset(context.Background()); err != nil {
		return fmt.Errorf("Error resetting users: %v", err)
	}
	fmt.Println("All users have been reset.")
	return nil
}

func handlerGetUsers(s *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("Expected no arguments after 'users' command")
	}
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting users: %v", err)
	}
	fmt.Println("Registered users:")
	for _, user := range users {
		if user.Name == s.cfg.Current_user_name {
			fmt.Printf("* %s (current)\n", user.Name)
			continue
		}
		fmt.Printf("* %s\n", user.Name)
	}
	return nil
}

// runs a given command with the provided state if it exists.
func (c *CLIcommands) run(s *state, cmd command) error {
	if handler, exists := c.cmd[cmd.name]; exists {
		return handler(s, cmd)
	} else {
		return fmt.Errorf("Unknown command: %s", cmd.name)
	}
}

// registers a new handler function for a command name
func (c *CLIcommands) register(name string, f func(*state, command) error) {
	c.cmd[name] = f
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatal("Error reading config:", err)
	}

	db, err := sql.Open("postgres", cfg.Db_url)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	dbQueries := database.New(db)

	s := &state{
		db:  dbQueries,
		cfg: &cfg,
	}
	cliCommands := &CLIcommands{cmd: make(map[string]func(*state, command) error)}
	cliCommands.register("login", handlerLogin)
	cliCommands.register("register", handlerRegister)
	cliCommands.register("reset", handlerReset)
	cliCommands.register("users", handlerGetUsers)

	cliArgs := os.Args[1:]
	/*if len(cliArgs) < 2 {
		log.Fatal(fmt.Errorf(
			"Expected at least 2 arguments: command name and its arguments"))
	}*/

	cmd := command{
		name: cliArgs[0],
		args: cliArgs[1:],
	}
	if err := cliCommands.run(s, cmd); err != nil {
		log.Fatal(err)
	}
}
