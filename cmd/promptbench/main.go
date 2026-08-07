// Command promptbench scores the article-analysis prompt against real collected articles.
//
// It exists because prompt work is otherwise judged by re-collecting and eyeballing the
// dashboard, which cannot separate a real improvement from a regression, and cannot compare
// two models at all. The bench runs the production analysis path, so what it measures is
// what collection would store.
//
//	go run ./cmd/promptbench -base-url http://127.0.0.1:8888/v1 -model gpt-5.6-luna
//	go run ./cmd/promptbench -base-url http://127.0.0.1:8888/v1 -model gpt-5.4-mini
//	go run ./cmd/promptbench -base-url http://127.0.0.1:8989/v1 -model Gemma-4-26B-A4B -repeat 3
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/found-cake/cyber-dashboard/internal/database"
	"github.com/found-cake/cyber-dashboard/internal/feed"
	"github.com/found-cake/cyber-dashboard/internal/summary"
)

type options struct {
	baseURL, model, apiKey, language, dataDir, dumpPath string
	days                                                string
	limit, repeat                                       int
	timeout                                             time.Duration
}

type observation struct {
	ArticleID int                       `json:"article_id"`
	Title     string                    `json:"title"`
	Runs      []summary.ArticleAnalysis `json:"runs"`
	Errors    []string                  `json:"errors,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "promptbench:", err)
		os.Exit(1)
	}
}

func run() error {
	opts := parseFlags()
	if strings.TrimSpace(opts.baseURL) == "" || strings.TrimSpace(opts.model) == "" {
		return fmt.Errorf("-base-url and -model are required")
	}
	client, err := summary.NewClient(summary.Config{
		BaseURL: opts.baseURL, Model: opts.model, APIKey: opts.apiKey, Timeout: opts.timeout,
	})
	if err != nil {
		return err
	}
	ctx := context.Background()
	articles, err := loadArticles(ctx, opts)
	if err != nil {
		return err
	}
	if len(articles) == 0 {
		return fmt.Errorf("no collected articles with a body were found; collect a day first")
	}
	fmt.Printf("model=%s  articles=%d  repeat=%d  language=%s\n\n", opts.model, len(articles), opts.repeat, opts.language)

	observations := make([]observation, 0, len(articles))
	for index, article := range articles {
		record := observation{ArticleID: int(article.ID), Title: article.Title}
		for attempt := 0; attempt < opts.repeat; attempt++ {
			analysis, analyzeErr := client.AnalyzeArticle(ctx, summary.ArticleRequest{
				Language: opts.language, Title: article.Title, URL: article.URL, Body: article.Body,
			})
			if analyzeErr != nil {
				record.Errors = append(record.Errors, analyzeErr.Error())
				continue
			}
			record.Runs = append(record.Runs, analysis)
		}
		observations = append(observations, record)
		fmt.Fprintf(os.Stderr, "\r  analyzed %d/%d", index+1, len(articles))
	}
	fmt.Fprintln(os.Stderr)

	report(observations, opts)
	if opts.dumpPath != "" {
		if err := dump(opts.dumpPath, observations); err != nil {
			return err
		}
		fmt.Printf("\nper-article results written to %s\n", opts.dumpPath)
	}
	return nil
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.baseURL, "base-url", "", "OpenAI-compatible base URL, up to but not including /chat/completions")
	flag.StringVar(&opts.model, "model", "", "model name to score")
	flag.StringVar(&opts.apiKey, "api-key", "", "API key, when the endpoint needs one")
	flag.StringVar(&opts.language, "language", "ko", "output language passed to the prompt (ko or en)")
	flag.StringVar(&opts.dataDir, "data-dir", defaultDataDir(), "directory holding dashboard.db")
	flag.StringVar(&opts.days, "days", "", "comma-separated YYYY-MM-DD days; defaults to every collected day")
	flag.StringVar(&opts.dumpPath, "dump", "", "write per-article results to this JSON file for diffing runs")
	flag.IntVar(&opts.limit, "limit", 0, "score at most this many articles (0 means all)")
	flag.IntVar(&opts.repeat, "repeat", 1, "analyze each article this many times to measure self-consistency")
	flag.DurationVar(&opts.timeout, "timeout", 120*time.Second, "per-request timeout")
	flag.Parse()
	if opts.repeat < 1 {
		opts.repeat = 1
	}
	return opts
}

func defaultDataDir() string {
	if configured := os.Getenv("CYBER_DASHBOARD_DATA_DIR"); configured != "" {
		return configured
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(configDir, "cyber-dashboard")
}

func loadArticles(ctx context.Context, opts options) ([]feed.ArticleForAnalysis, error) {
	db, err := database.Open(ctx, filepath.Join(opts.dataDir, "dashboard.db"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	repository := feed.NewRepository(db)
	days := splitDays(opts.days)
	if len(days) == 0 {
		days, err = repository.CollectedDays(ctx)
		if err != nil {
			return nil, err
		}
	}
	var articles []feed.ArticleForAnalysis
	for _, day := range days {
		dayArticles, dayErr := repository.ArticlesForAnalysis(ctx, day)
		if dayErr != nil {
			return nil, dayErr
		}
		articles = append(articles, dayArticles...)
		if opts.limit > 0 && len(articles) >= opts.limit {
			return articles[:opts.limit], nil
		}
	}
	return articles, nil
}

func splitDays(value string) []string {
	var days []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			days = append(days, trimmed)
		}
	}
	return days
}

// scorecard holds every number the report prints. Scoring is kept separate from printing so
// the measurements this tool exists to produce can themselves be tested.
type scorecard struct {
	observations, analyzed, failed                      int
	offEnumMethod, offEnumSector                        int
	sentinelMismatch, languageLeak, countryFromLanguage int
	unstableMethod, unstableActor                       int
	damageStated, flawArticles, patchStated             int
	none, unknown, unidentified, languageOnly, named    int
	aiOperated                                          int
	distinctNamed                                       int
	methods, actors                                     map[string]int
	singletonActors                                     []string
}

func score(observations []observation) scorecard {
	card := scorecard{observations: len(observations), methods: map[string]int{}, actors: map[string]int{}}

	for _, record := range observations {
		card.failed += len(record.Errors)
		methodSet, actorSet := map[string]bool{}, map[string]bool{}
		for _, analysis := range record.Runs {
			card.analyzed++
			card.methods[analysis.AttackMethod]++
			card.actors[analysis.ThreatActor]++
			methodSet[analysis.AttackMethod] = true
			actorSet[analysis.ThreatActor] = true

			if !isAttackMethod(analysis.AttackMethod) {
				card.offEnumMethod++
			}
			if !isTargetSector(analysis.TargetSector) {
				card.offEnumSector++
			}
			// Severity reads damage_usd, so how often the model finds a figure at all is
			// worth watching: a run where it never does is a prompt problem, not quiet news.
			if analysis.DamageUSD > 0 {
				card.damageStated++
			}
			// Severity moves a step on the patch state, so a blank one on an article about
			// a specific flaw is a miss. Articles that are not about a flaw are left out of
			// the denominator: having no patch state is the right answer there.
			if isFlawMethod(analysis.AttackMethod) {
				card.flawArticles++
				if analysis.PatchAvailable != "" {
					card.patchStated++
				}
			}
			// The prompt pins these two together: every non-incident label means no actor,
			// and an incident always carries some actor value. Only labels inside the
			// taxonomy are judged here, so an off-enum method is not counted twice.
			if isAttackMethod(analysis.AttackMethod) &&
				isIncidentMethod(analysis.AttackMethod) == (analysis.ThreatActor == noAttack) {
				card.sentinelMismatch++
			}
			// A Korean run must still emit the English sentinels, or the dashboard shows
			// "Unknown" and its translation as two separate bars.
			if containsHangul(analysis.AttackMethod) || containsHangul(analysis.ThreatActor) || containsHangul(analysis.TargetSector) {
				card.languageLeak++
			}
			// Language community is not nationality.
			if isLanguageOnlyActor(analysis.ThreatActor) && analysis.ActorCountry != "" {
				card.countryFromLanguage++
			}
		}
		if len(methodSet) > 1 {
			card.unstableMethod++
		}
		if len(actorSet) > 1 {
			card.unstableActor++
		}
	}

	singletons := map[string]int{}
	for actor, count := range card.actors {
		switch {
		case actor == noAttack:
			card.none += count
		case actor == unknownActor:
			card.unknown += count
		case actor == aiOperatedActor:
			card.aiOperated += count
		case isUnidentifiedActor(actor):
			card.unidentified += count
		case isLanguageOnlyActor(actor):
			card.languageOnly += count
		default:
			card.named += count
			card.distinctNamed++
			if count == 1 {
				singletons[actor] = count
			}
		}
	}
	card.singletonActors = sortedKeys(singletons)
	return card
}

func report(observations []observation, opts options) {
	card := score(observations)

	fmt.Println("── conformance ──────────────────────────────")
	line("analyses completed", card.analyzed, card.analyzed)
	if card.failed > 0 {
		fmt.Printf("  %-34s %d\n", "request failures", card.failed)
	}
	line("attack_method in taxonomy", card.analyzed-card.offEnumMethod, card.analyzed)
	line("target_sector in taxonomy", card.analyzed-card.offEnumSector, card.analyzed)
	line("None paired across the two fields", card.analyzed-card.sentinelMismatch, card.analyzed)
	line("labels kept in English", card.analyzed-card.languageLeak, card.analyzed)
	line("language community not made a country", card.analyzed-card.countryFromLanguage, card.analyzed)
	line("damage figure captured", card.damageStated, card.analyzed)
	line("patch state on flaw articles", card.patchStated, card.flawArticles)

	if opts.repeat > 1 {
		fmt.Println("\n── self-consistency across repeats ──────────")
		line("attack_method stable", card.observations-card.unstableMethod, card.observations)
		line("threat_actor stable", card.observations-card.unstableActor, card.observations)
	}

	fmt.Println("\n── attack_method distribution ───────────────")
	printCounts(card.methods, card.analyzed)

	fmt.Println("\n── threat_actor shape ───────────────────────")
	line("no incident (None)", card.none, card.analyzed)
	line("unattributed (Unknown)", card.unknown, card.analyzed)
	line("country-linked placeholder", card.unidentified, card.analyzed)
	line("language-community placeholder", card.languageOnly, card.analyzed)
	line("AI-operated placeholder", card.aiOperated, card.analyzed)
	line("attributed group", card.named, card.analyzed)
	fmt.Printf("  %-34s %d of %d distinct named actors\n", "seen exactly once", len(card.singletonActors), card.distinctNamed)
	if len(card.singletonActors) > 0 {
		fmt.Println("  review these for campaign, malware, kit, or researcher names:")
		for _, actor := range card.singletonActors {
			fmt.Printf("    - %s\n", actor)
		}
	}
}

func line(label string, value, total int) {
	if total == 0 {
		fmt.Printf("  %-34s %d\n", label, value)
		return
	}
	fmt.Printf("  %-34s %4d / %-4d  %5.1f%%\n", label, value, total, float64(value)*100/float64(total))
}

func printCounts(counts map[string]int, total int) {
	type entry struct {
		label string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for label, count := range counts {
		entries = append(entries, entry{label, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].label < entries[j].label
	})
	for _, item := range entries {
		fmt.Printf("  %-34s %4d  %5.1f%%\n", item.label, item.count, float64(item.count)*100/float64(max(total, 1)))
	}
}

func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsHangul(value string) bool {
	for _, symbol := range value {
		if symbol >= 0xAC00 && symbol <= 0xD7A3 {
			return true
		}
	}
	return false
}

func dump(path string, observations []observation) error {
	encoded, err := json.MarshalIndent(observations, "", "  ")
	if err != nil {
		return fmt.Errorf("encode results: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	return nil
}
