package services

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"time"

	app "github.com/CamilleOnoda/gator/internal/app"
	"github.com/CamilleOnoda/gator/internal/config"
	"github.com/CamilleOnoda/gator/internal/database"
	"github.com/google/uuid"
)

func fetchFeed(ctx context.Context, feedURL string) (*config.RSSFeed, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating request: %v", err)
	}
	req.Header.Set("User-Agent", "gator")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error performing request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %v", err)
	}

	var rssFeed config.RSSFeed
	if err := xml.Unmarshal(body, &rssFeed); err != nil {
		return nil, fmt.Errorf("Error unmarshalling RSS feed: %v", err)
	}

	unescapedTitle := html.UnescapeString(rssFeed.Channel.Title)
	rssFeed.Channel.Title = unescapedTitle
	unescapedDescription := html.UnescapeString(rssFeed.Channel.Description)
	rssFeed.Channel.Description = unescapedDescription
	for _, item := range rssFeed.Channel.Item {
		unescapedItemTitle := html.UnescapeString(item.Title)
		item.Title = unescapedItemTitle
		unescapedItemDescription := html.UnescapeString(item.Description)
		item.Description = unescapedItemDescription
	}

	return &rssFeed, nil
}

func ScrapeFeeds(s *app.State) error {
	feed, err := s.Db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("error getting next feed to fetch: %v", err)
	}
	if feed.ID == uuid.Nil {
		fmt.Println("No feeds to fetch")
		return nil
	}
	if err := s.Db.MarkFeedFetched(context.Background(), feed.ID); err != nil {
		return fmt.Errorf("Error marking the feed as fetched: %v", err)
	}

	rssFeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		return fmt.Errorf("Error fetching feed: %v", err)
	}

	for _, item := range rssFeed.Channel.Item {
		formats := []string{time.RFC1123Z, time.RFC1123}
		var pubDate time.Time
		for _, format := range formats {
			pubDate, err = time.Parse(format, item.PubDate)
			if err == nil {
				break
			}
		}
		post, err := s.Db.CreatePost(context.Background(), database.CreatePostParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Title:     item.Title,
			Url:       item.Link,
			Description: sql.NullString{
				String: item.Description,
				Valid:  item.Description != ""},
			PublishedAt: pubDate,
			FeedID:      feed.ID,
		})
		if err != nil {
			log.Printf("Error creating post: %v", err)
		} else {
			log.Printf("Post created: %s", post.Title)
		}
	}

	return nil
}
