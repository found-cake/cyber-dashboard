package summary

// The analysis prompt hands back classification labels that severity has to reason about,
// so the two facts it needs are stated here, beside the prompt that offers the labels.
// cmd/promptbench deliberately keeps its own copy of the full label lists: its job is to
// check the prompt against an independent list, which a shared one could not do.

// The two threat_actor sentinels the prompt pins to English so a Korean run does not split
// them into separate bars on the dashboard.
const (
	noAttackActor = "None"
	unknownActor  = "Unknown"
)

// nonIncidentMethods are the attack_method labels for an article that reports no real
// attack. They are the ones AnalyzeArticleSystemPrompt pairs with a "None" threat actor.
var nonIncidentMethods = map[string]bool{
	"Vulnerability Disclosure": true,
	"Industry / Guidance":      true,
	"None":                     true,
	// Articles collected before analysis carry the feed's own category, and articles the
	// analysis never reached keep the schema default. Neither states an incident.
	"Unclassified": true,
	"":             true,
}

// highImpactSectors are the target_sector labels where the same incident costs more than it
// would elsewhere, because the victims are people who cannot shop elsewhere while it is
// resolved: patients, citizens, and the services a region runs on.
var highImpactSectors = map[string]bool{
	"Government":              true,
	"Healthcare":              true,
	"Critical Infrastructure": true,
}

// IsIncidentMethod reports whether the label describes an attack that actually happened.
func IsIncidentMethod(attackMethod string) bool { return !nonIncidentMethods[attackMethod] }

// IsHighImpactSector reports whether an incident against this sector carries more weight.
func IsHighImpactSector(targetSector string) bool { return highImpactSectors[targetSector] }
