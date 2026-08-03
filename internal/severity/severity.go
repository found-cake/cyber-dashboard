package severity

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

func Max(levels ...Level) Level {
	selected := Unknown
	for _, level := range levels {
		if rank(level) > rank(selected) {
			selected = level
		}
	}
	return selected
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
