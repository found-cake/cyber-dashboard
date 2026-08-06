package main

import "strings"

// attackMethodLabels is the closed set of attack_method labels the analysis prompt allows.
// The dashboard groups by exact string, so any value outside this set becomes its own bar
// and fragments the distribution.
var attackMethodLabels = []string{
	"APT / Espionage",
	"Supply Chain",
	"Malware / Stealer",
	"Ransomware",
	"Botnet",
	"Financial / Crypto",
	"Social Engineering",
	"Vulnerability Exploitation",
	"Denial of Service",
	"Insider Threat",
	"Data Breach / Unauthorized Access",
	"Vulnerability Disclosure",
	"Industry / Guidance",
	"None",
}

// nonIncidentLabels are the attack_method labels that describe an article with no real
// attack in it. They exist because a single "None" bucket was the largest bar on the chart
// while merging a critical unpatched advisory with a vendor press release. Every one of them
// forces threat_actor to noAttack.
var nonIncidentLabels = []string{
	"Vulnerability Disclosure",
	"Industry / Guidance",
	"None",
}

// targetSectorLabels is the closed set of target_sector labels the analysis prompt allows.
var targetSectorLabels = []string{
	"Government",
	"Finance",
	"Technology",
	"Telecommunications",
	"Healthcare",
	"Education / Research",
	"Manufacturing",
	"Critical Infrastructure",
	"Retail / Consumer",
	"Media / Entertainment",
	"General",
}

// noAttack and unknownActor are the two sentinel values the prompt pins to English so a
// Korean run does not split them into separate bars.
const (
	noAttack     = "None"
	unknownActor = "Unknown"
)

// isAttackMethod reports whether label is one of the allowed attack_method values.
func isAttackMethod(label string) bool { return contains(attackMethodLabels, label) }

// isIncidentMethod reports whether the label describes a real attack. Callers pair it with
// threat_actor: a non-incident article has no attacker, so its actor must be noAttack.
func isIncidentMethod(label string) bool {
	return isAttackMethod(label) && !contains(nonIncidentLabels, label)
}

// isTargetSector reports whether label is one of the allowed target_sector values.
func isTargetSector(label string) bool { return contains(targetSectorLabels, label) }

// isUnidentifiedActor matches the "Unidentified {Country}-linked actor" form the prompt
// uses when an attack is tied to a state but no group is named.
func isUnidentifiedActor(actor string) bool {
	return strings.HasPrefix(actor, "Unidentified ") && strings.HasSuffix(actor, "-linked actor")
}

// isLanguageOnlyActor matches the "Unknown ({Language}-speaking)" form, which records a
// language community without claiming a nationality.
func isLanguageOnlyActor(actor string) bool {
	return strings.HasPrefix(actor, "Unknown (") && strings.HasSuffix(actor, "-speaking)")
}

// isNamedActor reports whether the actor is an attributed group rather than one of the
// sentinel or evidence-preserving placeholder forms.
func isNamedActor(actor string) bool {
	switch {
	case actor == "", actor == noAttack, actor == unknownActor:
		return false
	case isUnidentifiedActor(actor), isLanguageOnlyActor(actor):
		return false
	default:
		return true
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
