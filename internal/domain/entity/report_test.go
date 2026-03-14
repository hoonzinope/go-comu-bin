package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReportTargetType(t *testing.T) {
	targetType, ok := ParseReportTargetType("post")
	require.True(t, ok)
	assert.Equal(t, ReportTargetPost, targetType)

	_, ok = ParseReportTargetType("user")
	assert.False(t, ok)
}

func TestParseReportReasonCode(t *testing.T) {
	reason, ok := ParseReportReasonCode("abuse")
	require.True(t, ok)
	assert.Equal(t, ReportReasonAbuse, reason)

	_, ok = ParseReportReasonCode("unknown")
	assert.False(t, ok)
}

func TestReport_Resolve(t *testing.T) {
	report := NewReport(ReportTargetPost, 10, 20, ReportReasonSpam, "detail")
	require.Equal(t, ReportStatusPending, report.Status)
	require.Nil(t, report.ResolvedBy)
	require.Nil(t, report.ResolvedAt)

	ok := report.Resolve(ReportStatusAccepted, "handled", 99)
	require.True(t, ok)
	assert.Equal(t, ReportStatusAccepted, report.Status)
	assert.Equal(t, "handled", report.ResolutionNote)
	require.NotNil(t, report.ResolvedBy)
	assert.Equal(t, int64(99), *report.ResolvedBy)
	assert.NotNil(t, report.ResolvedAt)

	ok = report.Resolve(ReportStatusPending, "invalid", 100)
	assert.False(t, ok)
	assert.Equal(t, ReportStatusAccepted, report.Status)

	ok = report.Resolve(ReportStatusRejected, "re-review", 101)
	assert.False(t, ok)
	assert.Equal(t, ReportStatusAccepted, report.Status)
}
