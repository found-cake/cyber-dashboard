package summary

import (
	"encoding/json"
	"math"
	"math/big"
	"regexp"
	"strings"
)

const (
	dataVolumeNumberPattern = `(?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]+)(?:\.[0-9]+)?`
	dataVolumeUnitPattern   = `(?:pebibytes?|petabytes?|페타바이트|tebibytes?|terabytes?|테라바이트|gibibytes?|gigabytes?|기가바이트|mebibytes?|megabytes?|메가바이트|kibibytes?|kilobytes?|킬로바이트|bytes?|바이트|pib|pb|tib|tb|gib|gb|mib|mb|kib|kb|b)`
)

var dataVolumePattern = regexp.MustCompile(`(?i)^\s*(` + dataVolumeNumberPattern + `)(?:\s*-\s*(` + dataVolumeNumberPattern + `))?\s*(` + dataVolumeUnitPattern + `)\s*$`)

var dataVolumeFactors = map[string]int64{
	"pebibytes": 1 << 50, "pebibyte": 1 << 50, "petabytes": 1e15, "petabyte": 1e15, "페타바이트": 1e15,
	"tebibytes": 1 << 40, "tebibyte": 1 << 40, "terabytes": 1e12, "terabyte": 1e12, "테라바이트": 1e12,
	"gibibytes": 1 << 30, "gibibyte": 1 << 30, "gigabytes": 1e9, "gigabyte": 1e9, "기가바이트": 1e9,
	"mebibytes": 1 << 20, "mebibyte": 1 << 20, "megabytes": 1e6, "megabyte": 1e6, "메가바이트": 1e6,
	"kibibytes": 1 << 10, "kibibyte": 1 << 10, "kilobytes": 1e3, "kilobyte": 1e3, "킬로바이트": 1e3,
	"bytes": 1, "byte": 1, "바이트": 1,
	"pib": 1 << 50, "pb": 1e15, "tib": 1 << 40, "tb": 1e12, "gib": 1 << 30, "gb": 1e9,
	"mib": 1 << 20, "mb": 1e6, "kib": 1 << 10, "kb": 1e3, "b": 1,
}

type dataVolume int64

func (v *dataVolume) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		*v = 0
		return nil
	}
	*v = dataVolume(parseDataVolumeText(value))
	return nil
}

func parseDataVolumeText(value string) int64 {
	matches := dataVolumePattern.FindStringSubmatch(value)
	if matches == nil {
		return 0
	}
	factor := dataVolumeFactors[strings.ToLower(matches[3])]
	lower, ok := parseDataVolumeNumber(matches[1])
	if !ok {
		return 0
	}
	if matches[2] == "" {
		bytes, ok := dataVolumeBytes(lower, factor)
		if !ok {
			return 0
		}
		return bytes
	}
	upper, ok := parseDataVolumeNumber(matches[2])
	if !ok || lower.Cmp(upper) > 0 {
		return 0
	}
	lowerBytes, lowerOK := dataVolumeBytes(lower, factor)
	_, upperOK := dataVolumeBytes(upper, factor)
	if !lowerOK || !upperOK {
		return 0
	}
	return lowerBytes
}

func parseDataVolumeNumber(value string) (*big.Rat, bool) {
	return new(big.Rat).SetString(strings.ReplaceAll(value, ",", ""))
}

func dataVolumeBytes(number *big.Rat, factor int64) (int64, bool) {
	if factor <= 0 {
		return 0, false
	}
	scaled := new(big.Rat).Mul(number, new(big.Rat).SetInt64(factor))
	limit := new(big.Int).Mul(big.NewInt(math.MaxInt64), scaled.Denom())
	if scaled.Num().Cmp(limit) > 0 {
		return 0, false
	}
	bytes := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	return bytes.Int64(), true
}
