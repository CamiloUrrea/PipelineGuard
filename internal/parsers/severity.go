package parsers

// SeverityRank maps the unified severity enum onto a comparable rank, highest
// severity first: CRITICAL=4, HIGH=3, MEDIUM=2, LOW=1, INFO=0.
//
// Any unrecognized value ranks 0 (same as INFO): unknown severity is treated as
// the least severe, never the most severe, so an unexpected value can't trip a
// build failure.
func SeverityRank(severity string) int {
	switch severity {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MEDIUM":
		return 2
	case "LOW":
		return 1
	case "INFO":
		return 0
	default:
		return 0
	}
}
