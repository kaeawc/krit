package output

import "github.com/kaeawc/krit/internal/scanner"

type ScoreWeights struct {
	Error   int `json:"error"`
	Warning int `json:"warning"`
	Info    int `json:"info"`
}

type ScoreSummary struct {
	Total int `json:"total"`
	Files int `json:"files"`
	Rules int `json:"rules"`
}

type ScoreReport struct {
	Score              int            `json:"score"`
	Grade              string         `json:"grade"`
	FindingsBySeverity map[string]int `json:"findingsBySeverity"`
	Weights            ScoreWeights   `json:"weights"`
	Summary            ScoreSummary   `json:"summary"`
}

var DefaultScoreWeights = ScoreWeights{Error: 100, Warning: 10, Info: 1}

func ScoreFindings(findings []scanner.Finding, files, rules int) ScoreReport {
	counts := map[string]int{"error": 0, "warning": 0, "info": 0}
	for _, finding := range findings {
		switch finding.Severity {
		case "error":
			counts["error"]++
		case "warning":
			counts["warning"]++
		case "info":
			counts["info"]++
		}
	}
	score := counts["error"]*DefaultScoreWeights.Error +
		counts["warning"]*DefaultScoreWeights.Warning +
		counts["info"]*DefaultScoreWeights.Info
	return ScoreReport{
		Score:              score,
		Grade:              GradeForScore(score),
		FindingsBySeverity: counts,
		Weights:            DefaultScoreWeights,
		Summary: ScoreSummary{
			Total: len(findings),
			Files: files,
			Rules: rules,
		},
	}
}

func GradeForScore(score int) string {
	switch {
	case score <= 100:
		return "A"
	case score <= 1000:
		return "B"
	case score <= 5000:
		return "C"
	case score <= 10000:
		return "D"
	default:
		return "F"
	}
}
