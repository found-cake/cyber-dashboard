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

// FromVulnerability adjusts a CVSS severity for required footholds and explicitly unavailable
// patches. Available or unspecified patches do not lower it, and an unknown score stays unknown.
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

// Patch states reported by an article; only an explicitly unavailable patch changes severity.
const (
	PatchAvailable   = "yes"
	PatchUnavailable = "no"
)

// requiresFoothold detects local, physical, or privileged access, including CVSS v2 Au metrics.
func requiresFoothold(vector string) bool {
	for _, metric := range strings.Split(strings.ToUpper(strings.TrimSpace(vector)), "/") {
		switch strings.TrimSpace(metric) {
		case "AV:L", "AV:P", "PR:H", "AU:S", "AU:M":
			return true
		}
	}
	return false
}

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

// FromDamage grades stated incident damage in US dollars as an independent severity signal.
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

// Cap limits hypothetical CVSS severity for articles that report no actual attack.
func Cap(level, ceiling Level) Level {
	if rank(level) > rank(ceiling) {
		return ceiling
	}
	return level
}

// Raise increases a graded severity by one step and leaves Unknown unchanged.
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
