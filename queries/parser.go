package queries

import (
	"strings"
)

// countryNameToID maps lowercase country names to ISO alpha-2 codes
var countryNameToID = map[string]string{
	"nigeria":        "NG",
	"angola":         "AO",
	"kenya":          "KE",
	"ghana":          "GH",
	"south africa":   "ZA",
	"ethiopia":       "ET",
	"tanzania":       "TZ",
	"uganda":         "UG",
	"egypt":          "EG",
	"cameroon":       "CM",
	"ivory coast":    "CI",
	"senegal":        "SN",
	"zambia":         "ZM",
	"zimbabwe":       "ZW",
	"mozambique":     "MZ",
	"rwanda":         "RW",
	"mali":           "ML",
	"niger":          "NE",
	"chad":           "TD",
	"somalia":        "SO",
	"sudan":          "SD",
	"algeria":        "DZ",
	"morocco":        "MA",
	"tunisia":        "TN",
	"libya":          "LY",
	"united states":  "US",
	"usa":            "US",
	"united kingdom": "GB",
	"uk":             "GB",
	"canada":         "CA",
	"australia":      "AU",
	"germany":        "DE",
	"france":         "FR",
	"brazil":         "BR",
	"india":          "IN",
	"china":          "CN",
	"japan":          "JP",
	// extend as needed
}

type ParsedNLQuery struct {
	Gender    string
	AgeGroup  string
	MinAge    string
	MaxAge    string
	CountryID string
}

type CountryLookupFunc func(name string) (code string, ok bool)

// ParseNLQuery converts a plain English query string into ProfileQueryParams fields.
// Returns (ParsedNLQuery, true) on success, or (zero, false) if nothing interpretable was found.
func ParseNLQuery(q string, lookupCountry CountryLookupFunc) (ParsedNLQuery, bool) {
	normalized := strings.ToLower(strings.TrimSpace(q))
	if normalized == "" {
		return ParsedNLQuery{}, false
	}

	result := ParsedNLQuery{}
	matched := false

	// --- Gender ---
	hasMale := containsWord(normalized, "male") || containsWord(normalized, "males") || containsWord(normalized, "man") || containsWord(normalized, "men") || containsWord(normalized, "boy") || containsWord(normalized, "boys")
	hasFemale := containsWord(normalized, "female") || containsWord(normalized, "females") || containsWord(normalized, "woman") || containsWord(normalized, "women") || containsWord(normalized, "girl") || containsWord(normalized, "girls")

	// Only set gender when exactly one side is present
	if hasMale && !hasFemale {
		result.Gender = "male"
		matched = true
	} else if hasFemale && !hasMale {
		result.Gender = "female"
		matched = true
	}
	// both present → no gender filter (e.g. "male and female teenagers")

	// --- Age group keywords ---
	// "young" is a special alias: maps to min_age=16, max_age=24 (not a stored age_group)
	if containsWord(normalized, "young") {
		result.MinAge = "16"
		result.MaxAge = "24"
		matched = true
	}

	switch {
	case containsWord(normalized, "child") || containsWord(normalized, "children") || containsWord(normalized, "kid") || containsWord(normalized, "kids"):
		result.AgeGroup = "child"
		matched = true
	case containsWord(normalized, "teenager") || containsWord(normalized, "teenagers") || containsWord(normalized, "teen") || containsWord(normalized, "teens"):
		result.AgeGroup = "teenager"
		matched = true
	case containsWord(normalized, "adult") || containsWord(normalized, "adults"):
		result.AgeGroup = "adult"
		matched = true
	case containsWord(normalized, "senior") || containsWord(normalized, "seniors") || containsWord(normalized, "elderly") || containsWord(normalized, "old"):
		result.AgeGroup = "senior"
		matched = true
	}

	// --- Relative age expressions: "above N", "over N", "below N", "under N", "older than N", "younger than N" ---
	if age, ok := extractAgeAfter(normalized, []string{"above", "over", "older than"}); ok {
		result.MinAge = age
		matched = true
	}
	if age, ok := extractAgeAfter(normalized, []string{"below", "under", "younger than"}); ok {
		result.MaxAge = age
		matched = true
	}

	// --- Country: "from <country>" ---
	if countryID, ok := extractCountry(normalized, lookupCountry); ok {
		result.CountryID = countryID
		matched = true
	}

	return result, matched
}

// containsWord checks for a whole-word match so "males" doesn't match "male" mid-word.
func containsWord(s, word string) bool {
	for _, w := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '-' || r == '\t'
	}) {
		if w == word {
			return true
		}
	}
	return false
}

// extractAgeAfter looks for patterns like "above 30", "over 18", "older than 25".
func extractAgeAfter(s string, triggers []string) (string, bool) {
	for _, trigger := range triggers {
		idx := strings.Index(s, trigger)
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(s[idx+len(trigger):])
		// grab the leading numeric token
		numStr := ""
		for _, ch := range rest {
			if ch >= '0' && ch <= '9' {
				numStr += string(ch)
			} else {
				break
			}
		}
		if numStr != "" {
			return numStr, true
		}
	}
	return "", false
}

// extractCountry scans for "from <country name>" and returns the country code.
func extractCountry(s string, lookup CountryLookupFunc) (string, bool) {
	idx := strings.Index(s, "from ")
	if idx == -1 {
		return "", false
	}
	candidate := strings.TrimSpace(s[idx+5:])

	// Try longest match first (e.g. "south korea" before "south")
	bestKey := ""
	bestCode := ""
	words := strings.Fields(candidate)
	for length := len(words); length >= 1; length-- {
		phrase := strings.Join(words[:length], " ")
		if code, ok := lookup(phrase); ok && len(phrase) > len(bestKey) {
			bestKey = phrase
			bestCode = code
		}
	}
	if bestKey != "" {
		return bestCode, true
	}
	return "", false
}
