package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	app "github.com/CamilleOnoda/gator/internal/app"
	config "github.com/CamilleOnoda/gator/internal/config"
	"github.com/CamilleOnoda/gator/internal/database"
	services "github.com/CamilleOnoda/gator/internal/services"
	"github.com/google/uuid"
)

func MiddlewareLoggedIn(handler func(s *app.State, cmd app.Command, user database.User) error) func(*app.State, app.Command) error {
	return func(s *app.State, cmd app.Command) error {
		currentUser, err := s.Db.GetUser(context.Background(), s.Cfg.Current_user_name)
		if err != nil {
			return fmt.Errorf("Error getting user: %v", err)
		}
		return handler(s, cmd, currentUser)
	}
}

func HandlerLogin(s *app.State, cmd app.Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Expected to receive a username after 'login' command")
	}

	username := cmd.Args[0]
	if _, err := s.Db.GetUser(context.Background(), username); err != nil {
		log.Fatal("User does not exist, register first")
	}
	err := config.SetUser(username)
	if err != nil {
		return fmt.Errorf("Error setting user: %v", err)
	}

	s.Cfg.Current_user_name = username
	fmt.Println("User set to:", username)
	return nil
}

func HandlerRegister(s *app.State, cmd app.Command) error {
	if len(cmd.Args) == 0 {
		return fmt.Errorf("Expected to receive a username after 'register' command")
	}

	username := cmd.Args[0]
	if _, err := s.Db.GetUser(context.Background(), username); err == nil {
		log.Fatal("User already exists")
	}

	_, err := s.Db.CreateUser(context.Background(), database.CreateUserParams{
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

	s.Cfg.Current_user_name = username
	fmt.Println("User registered and set to:", username)

	return nil
}

func HandlerReset(s *app.State, cmd app.Command) error {
	if len(cmd.Args) != 0 {
		return fmt.Errorf("Expected no arguments after 'reset' command")
	}
	if err := s.Db.Reset(context.Background()); err != nil {
		return fmt.Errorf("Error resetting users: %v", err)
	}
	fmt.Println("All users have been reset.")
	return nil
}

func HandlerGetUsers(s *app.State, cmd app.Command) error {
	if len(cmd.Args) != 0 {
		return fmt.Errorf("Expected no arguments after 'users' command")
	}
	users, err := s.Db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting users: %v", err)
	}
	fmt.Println("Registered users:")
	for _, user := range users {
		if user.Name == s.Cfg.Current_user_name {
			fmt.Printf("* %s (current)\n", user.Name)
			continue
		}
		fmt.Printf("* %s\n", user.Name)
	}
	return nil
}

func HandlerAgg(s *app.State, cmd app.Command) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Expected to receive a time duration (e.g. '10s', '1m') after 'agg' command")
	}

	time_between_reqs, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return fmt.Errorf("Error parsing time duration: %v", err)
	}

	fmt.Printf("Collecting feeds every %s...\n", time_between_reqs)

	ticker := time.NewTicker(time_between_reqs)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		services.ScrapeFeeds(s)
	}
}

func HandlerAddFeed(s *app.State, cmd app.Command, user database.User) error {
	if len(cmd.Args) != 2 {
		return fmt.Errorf("Usage: addfeed <feed name> <feed url>")
	}
	feedName := cmd.Args[0]
	feedURL := cmd.Args[1]

	feed, err := s.Db.GetFeedByURL(context.Background(), feedURL)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("Error checking existing feed: %v", err)
	}

	if err == nil {
		_, followErr := s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			UserID:    user.ID,
			FeedID:    feed.ID,
		})
		if followErr == nil {
			fmt.Printf("Feed '%s' already exists; now followed by user '%s'\n", feed.Name, user.Name)
			return nil
		}

		if strings.Contains(strings.ToLower(followErr.Error()), "unique") {
			fmt.Printf("User '%s' already follows feed '%s'\n", user.Name, feed.Name)
			return nil
		}

		return fmt.Errorf("Error following existing feed: %v", followErr)
	}

	feed, err = s.Db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      feedName,
		Url:       feedURL,
		UserID:    user.ID,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return fmt.Errorf("Feed name or URL already exists. Choose a different name/url")
		}
		return fmt.Errorf("Error creating feed: %v", err)
	}

	_, err = s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return fmt.Errorf("Error following feed: %v", err)
	}

	fmt.Printf("Feed '%s' added successfully for user '%s'\n", feed.Name, user.Name)
	fmt.Printf("Feed url: %s\nCreated at: %s", feed.Url, feed.CreatedAt)
	return nil
}

func HandlerGetFeeds(s *app.State, cmd app.Command) error {
	if len(cmd.Args) != 0 {
		return fmt.Errorf("Expected no arguments after 'feeds' command")
	}

	feed, err := s.Db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("Error getting feeds: %v", err)
	}

	fmt.Println("Feeds:")
	for _, f := range feed {
		fmt.Printf("- %s (by %s): %s\n", f.Name, f.UserName, f.Url)
	}

	return nil
}

func HandlerFeedFollow(s *app.State, cmd app.Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Usage: follow <feed url>")
	}

	feedData, err := s.Db.GetFeedByURL(context.Background(), cmd.Args[0])
	if err != nil {
		return fmt.Errorf("Error getting feed: %v", err)
	}

	_, err = s.Db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feedData.ID,
	})
	if err != nil {
		return fmt.Errorf("Error following feed: %v", err)
	}

	fmt.Printf("Successfully followed feed: %s\nUser: %s\n", feedData.Name, user.Name)
	return nil
}

func HandlerFollowingFeeds(s *app.State, cmd app.Command, user database.User) error {
	if len(cmd.Args) != 0 {
		return fmt.Errorf("Usage: following")
	}

	feeds, err := s.Db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("Error getting followed feeds: %v", err)
	}

	fmt.Printf("Feeds followed by: %s\n", user.Name)
	for _, feed := range feeds {
		fmt.Printf("- %s\n", feed.FeedName)
	}
	return nil
}

func HandlerUnfollow(s *app.State, cmd app.Command, user database.User) error {
	if len(cmd.Args) != 1 {
		return fmt.Errorf("Usage: unfollow <feed url>")
	}
	feedURL := cmd.Args[0]
	feedData, err := s.Db.GetFeedByURL(context.Background(), feedURL)

	if err != nil {
		return fmt.Errorf("Error getting feed: %v", err)
	}
	err = s.Db.DeleteFeedFollow(context.Background(), database.DeleteFeedFollowParams{
		FeedID: feedData.ID,
		UserID: user.ID,
	})
	if err != nil {
		return fmt.Errorf("Error unfollowing feed: %v", err)
	}

	fmt.Printf("Successfully unfollowed feed: %s", feedData.Name)
	return nil

}

func HandlerBrowse(s *app.State, cmd app.Command, user database.User) error {
	limit := 2
	if len(cmd.Args) == 1 {
		num, err := strconv.Atoi(cmd.Args[0])
		if err != nil {
			return fmt.Errorf("Error parsing limit argument: %v", err)
		}
		limit = num
	} else if len(cmd.Args) > 1 {
		return fmt.Errorf("Usage: browse <limit>")
	}

	fmt.Println("Browsing posts...")

	posts, err := s.Db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	})
	if err != nil {
		return fmt.Errorf("Error getting posts: %v", err)
	}

	for i, post := range posts {
		fmt.Printf("Post %d:\n\n", i+1)
		fmt.Printf("Title: %s\n-- URL: %s\n-- Published: %v\n\n",
			post.Title, post.Url, post.PublishedAt)
	}
	return nil
}
