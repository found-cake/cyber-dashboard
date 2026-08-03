package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/found-cake/cyber-dashboard/api"
)

const baseURL = "https://raw.githubusercontent.com/found-cake/cyber-news-feed/master/data/rss/"

type Fetcher interface {
	Fetch(ctx context.Context, source api.Source) (Document, error)
}

type HTTPFetcher struct {
	client  *http.Client
	baseURL string
}

func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{client: &http.Client{Timeout: 20 * time.Second}, baseURL: baseURL}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, source api.Source) (Document, error) {
	endpoint, err := url.JoinPath(f.baseURL, source.Slug+".json")
	if err != nil {
		return Document{}, fmt.Errorf("build feed URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Document{}, fmt.Errorf("create feed request: %w", err)
	}
	request.Header.Set("User-Agent", "Cyber-Dashboard/1.0")
	response, err := f.client.Do(request)
	if err != nil {
		return Document{}, fmt.Errorf("fetch %s: %w", source.Slug, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Document{}, fmt.Errorf("fetch %s: status %d", source.Slug, response.StatusCode)
	}
	var document Document
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return Document{}, fmt.Errorf("decode %s feed: %w", source.Slug, err)
	}
	return document, nil
}

type Collector struct {
	repository *Repository
	fetcher    Fetcher
}

func NewCollector(repository *Repository, fetcher Fetcher) *Collector {
	return &Collector{repository: repository, fetcher: fetcher}
}

func (c *Collector) Collect(ctx context.Context, day string) (api.CollectionResult, error) {
	sources, err := c.repository.Sources(ctx)
	if err != nil {
		return api.CollectionResult{}, err
	}
	result := api.CollectionResult{Day: day, Warnings: []string{}}
	var mutex sync.Mutex
	var wait sync.WaitGroup
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		result.Sources++
		wait.Add(1)
		go func() {
			defer wait.Done()
			document, fetchErr := c.fetcher.Fetch(ctx, source)
			if fetchErr != nil {
				mutex.Lock()
				result.Warnings = append(result.Warnings, source.Name+": "+fetchErr.Error())
				mutex.Unlock()
				return
			}
			if !document.Status.OK && len(document.Articles) == 0 {
				mutex.Lock()
				result.Warnings = append(result.Warnings, source.Name+": stale feed")
				mutex.Unlock()
				return
			}
			for _, article := range document.Articles {
				articleDay, valid := publishedDay(article)
				if !valid || articleDay != day {
					continue
				}
				if saveErr := c.repository.SaveArticle(ctx, source, article, day); saveErr != nil {
					mutex.Lock()
					result.Warnings = append(result.Warnings, source.Name+": "+saveErr.Error())
					mutex.Unlock()
					continue
				}
				mutex.Lock()
				result.Collected++
				mutex.Unlock()
			}
		}()
	}
	wait.Wait()
	return result, nil
}

func publishedDay(article FeedArticle) (string, bool) {
	if article.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, article.PublishedAt); err == nil {
			return parsed.UTC().Format(time.DateOnly), true
		}
	}
	if strings.TrimSpace(article.PublishedRaw) != "" {
		if parsed, err := mail.ParseDate(article.PublishedRaw); err == nil {
			return parsed.UTC().Format(time.DateOnly), true
		}
	}
	return "", false
}
