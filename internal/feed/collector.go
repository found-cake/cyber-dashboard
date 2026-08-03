package feed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

type ArticleBodyProvider interface {
	Load(ctx context.Context, source api.Source, article FeedArticle) (string, error)
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
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return Document{}, fmt.Errorf("fetch %s: status %d; read response body: %w", source.Slug, response.StatusCode, readErr)
		}
		return Document{}, fmt.Errorf("fetch %s: status %d: %s", source.Slug, response.StatusCode, strings.TrimSpace(string(body)))
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
	bodyLoader ArticleBodyProvider
}

func NewCollector(repository *Repository, fetcher Fetcher, bodyLoader ArticleBodyProvider) *Collector {
	return &Collector{repository: repository, fetcher: fetcher, bodyLoader: bodyLoader}
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
				slog.WarnContext(ctx, "feed fetch failed", slog.String("source", source.Name), slog.String("response_body", fetchErr.Error()))
				mutex.Lock()
				result.Warnings = append(result.Warnings, source.Name+": 피드 수집 실패 / feed collection failed")
				mutex.Unlock()
				return
			}
			if !document.Status.OK && len(document.Articles) == 0 {
				mutex.Lock()
				result.Warnings = append(result.Warnings, source.Name+": 피드가 최신 상태가 아닙니다 / feed is stale")
				mutex.Unlock()
				return
			}
			for _, article := range document.Articles {
				articleDay, valid := publishedDay(article)
				if !valid || articleDay != day {
					continue
				}
				body, loadErr := c.bodyLoader.Load(ctx, source, article)
				if errors.Is(loadErr, ErrArticleFiltered) {
					continue
				}
				if loadErr != nil {
					slog.WarnContext(ctx, "article body fetch failed", slog.String("source", source.Name),
						slog.String("article_url", article.URL), slog.String("response_body", loadErr.Error()))
					mutex.Lock()
					result.Warnings = append(result.Warnings, source.Name+": 기사 본문 수집 실패 / article body collection failed")
					mutex.Unlock()
				} else {
					article.Body = body
				}
				if saveErr := c.repository.SaveArticle(ctx, source, article, day); saveErr != nil {
					slog.ErrorContext(ctx, "article save failed", slog.String("source", source.Name),
						slog.String("article_url", article.URL), slog.String("response_body", saveErr.Error()))
					mutex.Lock()
					result.Warnings = append(result.Warnings, source.Name+": 기사 저장 실패 / article save failed")
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
	parsed, valid := publishedTime(article)
	if !valid {
		return "", false
	}
	return parsed.Format(time.DateOnly), true
}

func publishedTimestamp(article FeedArticle, fallbackDay string) string {
	parsed, valid := publishedTime(article)
	if !valid {
		return fallbackDay + "T00:00:00Z"
	}
	return parsed.Format(time.RFC3339)
}

func publishedTime(article FeedArticle) (time.Time, bool) {
	if article.PublishedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, article.PublishedAt); err == nil {
			return parsed.UTC(), true
		}
	}
	if strings.TrimSpace(article.PublishedRaw) != "" {
		if parsed, err := mail.ParseDate(article.PublishedRaw); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
