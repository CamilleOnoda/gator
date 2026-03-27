package main

import (
	"database/sql"
	"log"
	"os"

	app "github.com/CamilleOnoda/gator/internal/app"
	"github.com/CamilleOnoda/gator/internal/config"
	"github.com/CamilleOnoda/gator/internal/database"
	handlers "github.com/CamilleOnoda/gator/internal/handlers"
	_ "github.com/lib/pq"
)

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

	s := &app.State{
		Db:  dbQueries,
		Cfg: &cfg,
	}
	cliCommands := &app.CLIcommands{Cmd: make(map[string]func(*app.State, app.Command) error)}
	cliCommands.Register("login", handlers.HandlerLogin)
	cliCommands.Register("register", handlers.HandlerRegister)
	cliCommands.Register("reset", handlers.HandlerReset)
	cliCommands.Register("users", handlers.HandlerGetUsers)
	cliCommands.Register("agg", handlers.HandlerAgg)
	cliCommands.Register("addfeed", handlers.MiddlewareLoggedIn(handlers.HandlerAddFeed))
	cliCommands.Register("feeds", handlers.HandlerGetFeeds)
	cliCommands.Register("follow", handlers.MiddlewareLoggedIn(handlers.HandlerFeedFollow))
	cliCommands.Register("following", handlers.MiddlewareLoggedIn(handlers.HandlerFollowingFeeds))
	cliCommands.Register("unfollow", handlers.MiddlewareLoggedIn(handlers.HandlerUnfollow))
	cliCommands.Register("browse", handlers.MiddlewareLoggedIn(handlers.HandlerBrowse))

	cliArgs := os.Args[1:]
	cmd := app.Command{
		Name: cliArgs[0],
		Args: cliArgs[1:],
	}
	if err := cliCommands.Run(s, cmd); err != nil {
		log.Fatal(err)
	}
}
