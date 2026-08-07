package severity

import "strings"

type Level string

const (
	Unknown  Level = "UNKNOWN"
	Low      Level = "LOW"
	Medium   Level = "MEDIUM"
	High     Level = "HIGH"
	Critical Level = "CRITICAL"
)

func FromCVSS(score float64) Level {
	switch {
	case score >= 9:
		return Critical
	case score >= 7:
		return High
	case score >= 4:
		return Medium
	case score > 0:
		return Low
	default:
		return Unknown
	}
}

// FromVulnerability grades a linked CVE from more than its base score. The score says how
// bad the flaw is once it is reached; the vector and the patch state say how likely reaching
// it is, and the score alone blurs both — a flaw needing physical access is scored like a
// remote one, and a flaw with no fix to install like one already patched. Neither adjustment
// can produce a level on its own: with no scored CVE there is nothing to adjust.
//
// An available fix does not lower the result. It once did, and on real collections that ran
// in one direction only: security writing reports the fix far more often than its absence,
// so nearly every scored article lost a step and the CRITICAL band emptied out while CVSS 9
// flaws sat in HIGH.
func FromVulnerability(score float64, vector, patchAvailable string) Level {
	level := FromCVSS(score)
	if level == Unknown {
		return Unknown
	}
	if requiresFoothold(vector) {
		level = shift(level, -1)
	}
	if strings.EqualFold(strings.TrimSpace(patchAvailable), PatchUnavailable) {
		level = shift(level, 1)
	}
	return level
}

// Patch states an article can report about the flaw it describes. Only PatchUnavailable
// moves severity: an article that never mentions a fix is not evidence that none exists.
const (
	PatchAvailable   = "yes"
	PatchUnavailable = "no"
)

// requiresFoothold reports whether the vector says the attacker must already be somewhere
// before the flaw is usable: at the machine, or holding administrative rights. CVSS v2
// spells authentication as Au, so both spellings are read.
func requiresFoothold(vector string) bool {
	for _, metric := range strings.Split(strings.ToUpper(strings.TrimSpace(vector)), "/") {
		switch strings.TrimSpace(metric) {
		case "AV:L", "AV:P", "PR:H", "AU:S", "AU:M":
			return true
		}
	}
	return false
}

// shift moves a level by steps without leaving the graded range: an adjustment refines a
// scored CVE, so it never erases it back to Unknown or past Critical.
func shift(level Level, steps int) Level {
	return fromRank(min(max(rank(level)+steps, rank(Low)), rank(Critical)))
}

func FromContext(victimCount int, zeroDay bool) Level {
	switch {
	case zeroDay, victimCount >= 100_000:
		return Critical
	case victimCount >= 1_000:
		return High
	case victimCount > 0:
		return Medium
	default:
		return Unknown
	}
}

// FromDamage grades the financial damage an article states for an incident, in US dollars.
// It is a signal of its own because a theft or an outage cost can be the only measure of an
// incident an article gives: a drained bridge or a wire-fraud loss often names no victim
// count and carries no CVE, and would otherwise be graded as if nothing had happened.
func FromDamage(damageUSD int64) Level {
	switch {
	case damageUSD >= 100_000_000:
		return Critical
	case damageUSD >= 10_000_000:
		return High
	case damageUSD >= 100_000:
		return Medium
	case damageUSD > 0:
		return Low
	default:
		return Unknown
	}
}

// Cap holds a level at a ceiling. It exists for articles that report no attack: their CVSS
// signal describes a flaw someone might exploit, not damage anyone has taken.
func Cap(level, ceiling Level) Level {
	if rank(level) > rank(ceiling) {
		return ceiling
	}
	return level
}

// Raise moves a graded level up one step, for a signal that sharpens an article already
// carrying severity rather than one that could grade it alone. Unknown stays Unknown.
func Raise(level Level) Level {
	if level == Unknown {
		return Unknown
	}
	return shift(level, 1)
}

func Max(levels ...Level) Level {
	selected := Unknown
	for _, level := range levels {
		if rank(level) > rank(selected) {
			selected = level
		}
	}
	return selected
}

func fromRank(value int) Level {
	switch value {
	case 4:
		return Critical
	case 3:
		return High
	case 2:
		return Medium
	case 1:
		return Low
	default:
		return Unknown
	}
}

func rank(level Level) int {
	switch level {
	case Critical:
		return 4
	case High:
		return 3
	case Medium:
		return 2
	case Low:
		return 1
	default:
		return 0
	}
}
