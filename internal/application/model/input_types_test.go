package model

import (
	"testing"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseReactionInputTypes(t *testing.T) {
	targetType, ok := ParseReactionTargetType("post")
	require.True(t, ok)
	entityTargetType, ok := targetType.ToEntity()
	require.True(t, ok)
	assert.Equal(t, entity.ReactionTargetPost, entityTargetType)

	reactionType, ok := ParseReactionType("like")
	require.True(t, ok)
	entityReactionType, ok := reactionType.ToEntity()
	require.True(t, ok)
	assert.Equal(t, entity.ReactionTypeLike, entityReactionType)
}

func TestParseReportInputTypes(t *testing.T) {
	targetType, ok := ParseReportTargetType("comment")
	require.True(t, ok)
	entityTargetType, ok := targetType.ToEntity()
	require.True(t, ok)
	assert.Equal(t, entity.ReportTargetComment, entityTargetType)

	reasonCode, ok := ParseReportReasonCode("abuse")
	require.True(t, ok)
	entityReasonCode, ok := reasonCode.ToEntity()
	require.True(t, ok)
	assert.Equal(t, entity.ReportReasonAbuse, entityReasonCode)

	status, ok := ParseReportStatus("accepted")
	require.True(t, ok)
	entityStatus, ok := status.ToEntity()
	require.True(t, ok)
	assert.Equal(t, entity.ReportStatusAccepted, entityStatus)
}

func TestParseSuspensionDuration(t *testing.T) {
	duration, ok := ParseSuspensionDuration("7d")
	require.True(t, ok)
	entityDuration, ok := duration.ToEntity()
	require.True(t, ok)
	assert.Equal(t, entity.SuspensionDuration7Days, entityDuration)
}

func TestInputTypeParsers_RejectInvalidValue(t *testing.T) {
	_, ok := ParseReactionType("invalid")
	assert.False(t, ok)
	_, ok = ParseReactionTargetType("invalid")
	assert.False(t, ok)
	_, ok = ParseReportTargetType("invalid")
	assert.False(t, ok)
	_, ok = ParseReportReasonCode("invalid")
	assert.False(t, ok)
	_, ok = ParseReportStatus("invalid")
	assert.False(t, ok)
	_, ok = ParseSuspensionDuration("invalid")
	assert.False(t, ok)
}
