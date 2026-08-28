package summary

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
)

const (
	dataVolumeNumberPattern = `(?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]+)(?:\.[0-9]+)?`
	dataVolumeUnitPattern   = `(?:(?i:pebibytes?|petabytes?|페타바이트|tebibytes?|terabytes?|테라바이트|gibibytes?|gigabytes?|기가바이트|mebibytes?|megabytes?|메가바이트|kibibytes?|kilobytes?|킬로바이트|bytes?|바이트)|P(?:i)?B|T(?:i)?B|G(?:i)?B|M(?:i)?B|K(?:i)?B|B|[Pp](?:[Ii])?b|[Tt](?:[Ii])?b|[Gg](?:[Ii])?b|[Mm](?:[Ii])?b|[Kk](?:[Ii])?b|b)`
)

var dataVolumePattern = regexp.MustCompile(`^\s*(` + dataVolumeNumberPattern + `)(?:\s*-\s*(` + dataVolumeNumberPattern + `))?\s*(` + dataVolumeUnitPattern + `)\s*$`)

var dataVolumeFactors = map[string]int64{
	"pebibytes": 1 << 50, "pebibyte": 1 << 50, "petabytes": 1e15, "petabyte": 1e15, "페타바이트": 1e15,
	"tebibytes": 1 << 40, "tebibyte": 1 << 40, "terabytes": 1e12, "terabyte": 1e12, "테라바이트": 1e12,
	"gibibytes": 1 << 30, "gibibyte": 1 << 30, "gigabytes": 1e9, "gigabyte": 1e9, "기가바이트": 1e9,
	"mebibytes": 1 << 20, "mebibyte": 1 << 20, "megabytes": 1e6, "megabyte": 1e6, "메가바이트": 1e6,
	"kibibytes": 1 << 10, "kibibyte": 1 << 10, "kilobytes": 1e3, "kilobyte": 1e3, "킬로바이트": 1e3,
	"bytes": 1, "byte": 1, "바이트": 1,
}

var dataVolumeAbbreviationFactors = map[string]int64{
	"": 1, "pi": 1 << 50, "p": 1e15, "ti": 1 << 40, "t": 1e12, "gi": 1 << 30, "g": 1e9,
	"mi": 1 << 20, "m": 1e6, "ki": 1 << 10, "k": 1e3,
}

var dataVolumeTextReplacer = strings.NewReplacer(
	"\u00a0", " ", "‐", "-", "‑", "-", "‒", "-", "–", "-", "—", "-", "−", "-",
)

type dataVolume struct {
	bytes int64
}

func (v *dataVolume) UnmarshalJSON(data []byte) error {
	if strings.TrimSpace(string(data)) == "null" {
		return fmt.Errorf("data_volume must be a string")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("data_volume must be a string: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		v.bytes = 0
		return nil
	}
	parsed, ok := parseDataVolumeText(value)
	if !ok {
		return fmt.Errorf("invalid data_volume %q", value)
	}
	v.bytes = parsed
	return nil
}

func parseDataVolumeText(value string) (int64, bool) {
	matches := dataVolumePattern.FindStringSubmatch(dataVolumeTextReplacer.Replace(value))
	if matches == nil {
		return 0, false
	}
	factor, divisor, ok := dataVolumeUnitScale(matches[3])
	if !ok {
		return 0, false
	}
	lower, ok := parseDataVolumeNumber(matches[1])
	if !ok {
		return 0, false
	}
	if matches[2] == "" {
		bytes, ok := dataVolumeBytes(lower, factor, divisor)
		if !ok {
			return 0, false
		}
		return bytes, true
	}
	upper, ok := parseDataVolumeNumber(matches[2])
	if !ok || lower.Cmp(upper) > 0 {
		return 0, false
	}
	lowerBytes, lowerOK := dataVolumeBytes(lower, factor, divisor)
	_, upperOK := dataVolumeBytes(upper, factor, divisor)
	if !lowerOK || !upperOK {
		return 0, false
	}
	return lowerBytes, true
}

func dataVolumeUnitScale(unit string) (int64, int64, bool) {
	if factor, ok := dataVolumeFactors[strings.ToLower(unit)]; ok {
		return factor, 1, true
	}
	factor, ok := dataVolumeAbbreviationFactors[strings.ToLower(unit[:len(unit)-1])]
	if !ok {
		return 0, 0, false
	}
	if unit[len(unit)-1] == 'b' {
		return factor, 8, true
	}
	return factor, 1, true
}

func parseDataVolumeNumber(value string) (*big.Rat, bool) {
	return new(big.Rat).SetString(strings.ReplaceAll(value, ",", ""))
}

func dataVolumeBytes(number *big.Rat, factor, divisor int64) (int64, bool) {
	if factor <= 0 || divisor <= 0 {
		return 0, false
	}
	scaled := new(big.Rat).Mul(number, new(big.Rat).SetFrac64(factor, divisor))
	limit := new(big.Int).Mul(big.NewInt(math.MaxInt64), scaled.Denom())
	if scaled.Num().Cmp(limit) > 0 {
		return 0, false
	}
	bytes := new(big.Int).Quo(scaled.Num(), scaled.Denom())
	return bytes.Int64(), true
}
