package api

// Confidence tier constants for rule-reported confidence scores.
// Values are preserved verbatim from the literals that previously
// appeared inline in each rule's Confidence() method; this file is a
// rename, not a re-tuning. Outlier values that did not fit a tier
// (e.g. 0.55, 0.65, 0.99) remain as literals at the call site.
const (
	ConfidenceLow           = 0.5
	ConfidenceMediumLow     = 0.6
	ConfidenceMediumLowPlus = 0.7
	ConfidenceMedium        = 0.75
	ConfidenceMediumHigh    = 0.8
	ConfidenceHigh          = 0.85
	ConfidenceHigher        = 0.9
	ConfidenceVeryHigh      = 0.95
)
