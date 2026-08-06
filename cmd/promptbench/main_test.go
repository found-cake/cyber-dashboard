package main

import (
	"reflect"
	"testing"

	"github.com/found-cake/cyber-dashboard/internal/summary"
)

func analysis(method, actor, country, sector string) summary.ArticleAnalysis {
	return summary.ArticleAnalysis{
		AttackMethod: method, ThreatActor: actor, ActorCountry: country, TargetSector: sector,
	}
}

func TestScoreCountsEveryContractViolation(t *testing.T) {
	// Given one analysis per violation the bench exists to catch, plus a clean one.
	observations := []observation{
		{Runs: []summary.ArticleAnalysis{analysis("Ransomware", "INC Ransomware", "", "Healthcare")}},
		{Runs: []summary.ArticleAnalysis{analysis("SQL 인젝션", "Unknown", "", "Technology")}},
		{Runs: []summary.ArticleAnalysis{analysis("Ransomware", "Unknown", "", "회사")}},
		{Runs: []summary.ArticleAnalysis{analysis("None", "Unknown", "", "General")}},
		{Runs: []summary.ArticleAnalysis{analysis("Vulnerability Disclosure", "Unknown", "", "Technology")}},
		{Runs: []summary.ArticleAnalysis{analysis("Botnet", "Unknown (Russian-speaking)", "Russia", "Technology")}},
		{Errors: []string{"request failed"}},
	}

	// When the run is scored.
	card := score(observations)

	// Then each violation is attributed to exactly the check that owns it.
	if card.analyzed != 6 || card.failed != 1 {
		t.Fatalf("analyzed = %d, failed = %d, want 6 and 1", card.analyzed, card.failed)
	}
	if card.offEnumMethod != 1 {
		t.Errorf("off-enum attack_method = %d, want 1", card.offEnumMethod)
	}
	if card.offEnumSector != 1 {
		t.Errorf("off-enum target_sector = %d, want 1", card.offEnumSector)
	}
	// "SQL 인젝션" and "회사" are both Hangul, so two analyses leak the output language.
	if card.languageLeak != 2 {
		t.Errorf("language leaks = %d, want 2", card.languageLeak)
	}
	// Both "None" and "Vulnerability Disclosure" are non-incident labels, so pairing them
	// with a threat_actor of "Unknown" breaks the rule twice. The off-enum method is not
	// judged here, so it is not counted a second time.
	if card.sentinelMismatch != 2 {
		t.Errorf("sentinel mismatches = %d, want 2", card.sentinelMismatch)
	}
	// A language community must not be promoted to a country.
	if card.countryFromLanguage != 1 {
		t.Errorf("country-from-language = %d, want 1", card.countryFromLanguage)
	}
}

func TestScoreBucketsActorForms(t *testing.T) {
	// Given one analysis per threat_actor form the prompt can produce.
	observations := []observation{
		{Runs: []summary.ArticleAnalysis{analysis("None", "None", "", "General")}},
		{Runs: []summary.ArticleAnalysis{analysis("Botnet", "Unknown", "", "Technology")}},
		{Runs: []summary.ArticleAnalysis{analysis("Botnet", "Unknown", "", "Technology")}},
		{Runs: []summary.ArticleAnalysis{analysis("APT / Espionage", "Unidentified China-linked actor", "China", "Government")}},
		{Runs: []summary.ArticleAnalysis{analysis("Ransomware", "Unknown (Chinese-speaking)", "", "Finance")}},
		{Runs: []summary.ArticleAnalysis{analysis("Ransomware", "Lazarus Group", "North Korea", "Finance")}},
		{Runs: []summary.ArticleAnalysis{analysis("Ransomware", "Lazarus Group", "North Korea", "Finance")}},
		{Runs: []summary.ArticleAnalysis{analysis("Malware / Stealer", "Shai-Hulud 2.0", "", "Technology")}},
	}

	// When the run is scored.
	card := score(observations)

	// Then placeholders stay out of the attributed-group count, which is what keeps the
	// dashboard's actor tail meaningful.
	if card.none != 1 || card.unknown != 2 || card.unidentified != 1 || card.languageOnly != 1 {
		t.Errorf("buckets none=%d unknown=%d unidentified=%d languageOnly=%d, want 1/2/1/1",
			card.none, card.unknown, card.unidentified, card.languageOnly)
	}
	if card.named != 3 || card.distinctNamed != 2 {
		t.Errorf("named = %d over %d distinct, want 3 over 2", card.named, card.distinctNamed)
	}
	// Only the once-seen actor is surfaced for review; a repeated one is not suspicious.
	if !reflect.DeepEqual(card.singletonActors, []string{"Shai-Hulud 2.0"}) {
		t.Errorf("singleton actors = %v, want [Shai-Hulud 2.0]", card.singletonActors)
	}
}

func TestScoreDetectsUnstableRepeats(t *testing.T) {
	// Given one article answered consistently and one that changes between repeats.
	observations := []observation{
		{Runs: []summary.ArticleAnalysis{
			analysis("Ransomware", "Unknown", "", "Finance"),
			analysis("Ransomware", "Unknown", "", "Finance"),
		}},
		{Runs: []summary.ArticleAnalysis{
			analysis("Supply Chain", "Unknown", "", "Technology"),
			analysis("Malware / Stealer", "Shai-Hulud 2.0", "", "Technology"),
		}},
	}

	// When the run is scored.
	card := score(observations)

	// Then only the article that changed counts as unstable, on each field separately.
	if card.unstableMethod != 1 {
		t.Errorf("unstable attack_method = %d, want 1", card.unstableMethod)
	}
	if card.unstableActor != 1 {
		t.Errorf("unstable threat_actor = %d, want 1", card.unstableActor)
	}
}

func TestSplitDaysIgnoresBlankEntries(t *testing.T) {
	// Given a day list with padding and empty segments, as a shell argument often has.
	// When it is split.
	days := splitDays(" 2026-08-04 ,, 2026-08-05 ,")

	// Then only real days survive.
	if !reflect.DeepEqual(days, []string{"2026-08-04", "2026-08-05"}) {
		t.Fatalf("splitDays = %v", days)
	}
	if got := splitDays(""); got != nil {
		t.Fatalf("splitDays(\"\") = %v, want nil so the caller falls back to every collected day", got)
	}
}

func TestContainsHangulSeparatesScripts(t *testing.T) {
	// Given labels in each script the analysis can return.
	// When they are checked for Hangul.
	// Then only Korean text is flagged, so English taxonomy labels never count as a leak.
	for _, value := range []string{"Ransomware", "Data Breach / Unauthorized Access", "None", ""} {
		if containsHangul(value) {
			t.Errorf("containsHangul(%q) = true, want false", value)
		}
	}
	for _, value := range []string{"랜섬웨어", "알 수 없음", "SQL 인젝션"} {
		if !containsHangul(value) {
			t.Errorf("containsHangul(%q) = false, want true", value)
		}
	}
}
