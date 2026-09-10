package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityRank_KnownValues(t *testing.T) {
	assert.Equal(t, 4, SeverityRank("CRITICAL"))
	assert.Equal(t, 3, SeverityRank("HIGH"))
	assert.Equal(t, 2, SeverityRank("MEDIUM"))
	assert.Equal(t, 1, SeverityRank("LOW"))
	assert.Equal(t, 0, SeverityRank("INFO"))
}

func TestSeverityRank_StrictOrdering(t *testing.T) {
	assert.Greater(t, SeverityRank("CRITICAL"), SeverityRank("HIGH"))
	assert.Greater(t, SeverityRank("HIGH"), SeverityRank("MEDIUM"))
	assert.Greater(t, SeverityRank("MEDIUM"), SeverityRank("LOW"))
	assert.Greater(t, SeverityRank("LOW"), SeverityRank("INFO"))
}

func TestSeverityRank_UnknownIsLeastSevere(t *testing.T) {
	assert.Equal(t, 0, SeverityRank("BANANA"))
	assert.Equal(t, 0, SeverityRank(""))
	assert.Equal(t, 0, SeverityRank("critical"), "match is case-sensitive")
}
