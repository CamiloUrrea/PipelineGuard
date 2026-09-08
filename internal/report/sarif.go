package report

import (
	"encoding/json"
	"fmt"

	"pipelineguard/internal/parsers"
)

// sarifSchemaURL is the canonical SARIF 2.1.0 schema URL that GitHub's Security
// tab expects in the "$schema" field.
const sarifSchemaURL = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"

// The sarif* structs below model just the subset of the SARIF 2.1.0 document
// PipelineGuard emits. They are unexported: callers only ever see the marshaled
// bytes from GenerateSARIF.

type sarifDocument struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name  string      `json:"name"`
	Rules []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string    `json:"id"`
	ShortDescription sarifText `json:"shortDescription"`
}

type sarifResult struct {
	RuleID     string           `json:"ruleId"`
	Level      string           `json:"level"`
	Message    sarifText        `json:"message"`
	Properties sarifResultProps `json:"properties"`
	Locations  []sarifLocation  `json:"locations"`
}

type sarifResultProps struct {
	OriginTool string `json:"originTool"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	// Region is omitted entirely when the finding's line is unknown (0):
	// SARIF requires startLine >= 1.
	Region *sarifRegion `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

type sarifText struct {
	Text string `json:"text"`
}

// sarifLevel maps PipelineGuard's unified severity enum onto a SARIF result
// level. An unrecognized severity falls back to "warning": a middle ground that
// neither hides the finding nor over-alarms.
func sarifLevel(severity string) string {
	switch severity {
	case "CRITICAL", "HIGH":
		return "error"
	case "MEDIUM":
		return "warning"
	case "LOW":
		return "note"
	case "INFO":
		return "none"
	default:
		return "warning"
	}
}

// GenerateSARIF renders the normalized findings as a SARIF 2.1.0 document, the
// standard OASIS format GitHub reads natively in the Security tab.
//
//   - rules is deduplicated by Finding.Rule; the shortDescription text comes
//     from the Message of the first finding carrying that rule.
//   - properties.originTool preserves which scanner produced each result.
//   - a finding with Line == 0 ("unknown") gets no region field.
//   - with no findings the document is still valid, with "results": [] and
//     "rules": [] (never null).
func GenerateSARIF(findings []parsers.Finding) ([]byte, error) {
	rules := make([]sarifRule, 0)
	seenRules := make(map[string]struct{})
	results := make([]sarifResult, 0, len(findings))

	for _, f := range findings {
		if _, seen := seenRules[f.Rule]; !seen {
			seenRules[f.Rule] = struct{}{}
			rules = append(rules, sarifRule{
				ID:               f.Rule,
				ShortDescription: sarifText{Text: f.Message},
			})
		}

		physical := sarifPhysicalLocation{
			ArtifactLocation: sarifArtifactLocation{URI: f.File},
		}
		if f.Line > 0 {
			physical.Region = &sarifRegion{StartLine: f.Line}
		}

		results = append(results, sarifResult{
			RuleID:     f.Rule,
			Level:      sarifLevel(f.Severity),
			Message:    sarifText{Text: f.Message},
			Properties: sarifResultProps{OriginTool: f.Tool},
			Locations:  []sarifLocation{{PhysicalLocation: physical}},
		})
	}

	doc := sarifDocument{
		Schema:  sarifSchemaURL,
		Version: "2.1.0",
		Runs: []sarifRun{
			{
				Tool: sarifTool{
					Driver: sarifDriver{
						Name:  "PipelineGuard",
						Rules: rules,
					},
				},
				Results: results,
			},
		},
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling SARIF document: %w", err)
	}
	return out, nil
}
