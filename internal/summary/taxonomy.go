package summary

// Keep sentinels in English to avoid localized dashboard buckets. promptbench intentionally
// maintains an independent taxonomy for validation.
const (
	noAttackActor = "None"
	unknownActor  = "Unknown"
)

// nonIncidentMethods are labels paired with a "None" threat actor.
var nonIncidentMethods = map[string]bool{
	"Vulnerability Disclosure": true,
	"Industry / Guidance":      true,
	"None":                     true,
	// Legacy feed categories and schema defaults do not state an incident.
	"Unclassified": true,
	"":             true,
}

// highImpactSectors identify essential services where the same incident has greater impact.
var highImpactSectors = map[string]bool{
	"Government":              true,
	"Healthcare":              true,
	"Critical Infrastructure": true,
}

// IsIncidentMethod reports whether the label describes an attack that actually happened.
func IsIncidentMethod(attackMethod string) bool { return !nonIncidentMethods[attackMethod] }

// IsHighImpactSector reports whether an incident against this sector carries more weight.
func IsHighImpactSector(targetSector string) bool { return highImpactSectors[targetSector] }
