package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestChangedEvents_ExposeEventNameAndOccurredAt(t *testing.T) {
	board := NewBoardChanged("created", 10)
	publishedAt := time.Now()
	post := NewPostChanged("updated", 20, 30, &publishedAt, []string{"go", "news"}, nil)
	comment := NewCommentChanged("deleted", 40, 20)
	reaction := NewReactionChanged("set", entity.ReactionTargetPost, 20, 20, 5, entity.ReactionTypeLike)
	attachment := NewAttachmentChanged("deleted", 50, 20)
	report := NewReportChanged("resolved", 77, "accepted")
	signup := NewSignupEmailVerificationRequested(1, "alice@example.com", "raw-signup", "hash-signup", time.Now().Add(time.Hour))
	resend := NewEmailVerificationResendRequested(1, "alice@example.com", "raw-resend", "hash-resend", time.Now().Add(time.Hour))
	reset := NewPasswordResetRequested(2, "bob@example.com", "raw-reset", "hash-reset", time.Now().Add(time.Hour))

	assert.Equal(t, EventNameBoardChanged, board.EventName())
	assert.Equal(t, EventNamePostChanged, post.EventName())
	assert.Equal(t, EventNameCommentChanged, comment.EventName())
	assert.Equal(t, EventNameReactionChanged, reaction.EventName())
	assert.Equal(t, EventNameAttachmentChanged, attachment.EventName())
	assert.Equal(t, EventNameReportChanged, report.EventName())
	assert.Equal(t, EventNameSignupEmailVerificationRequested, signup.EventName())
	assert.Equal(t, EventNameEmailVerificationResendRequested, resend.EventName())
	assert.Equal(t, EventNamePasswordResetRequested, reset.EventName())

	assert.WithinDuration(t, time.Now(), board.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), post.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), comment.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), reaction.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), attachment.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), report.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), signup.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), resend.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), reset.OccurredAt(), time.Second)
}

func TestPostChanged_JSONPayloadContainsMinimalFields(t *testing.T) {
	publishedAt := time.Now()
	e := NewPostChanged("deleted", 1, 2, &publishedAt, []string{"go"}, []int64{10, 11})

	payload, err := json.Marshal(e)
	assert.NoError(t, err)
	assert.Contains(t, string(payload), `"operation":"deleted"`)
	assert.Contains(t, string(payload), `"post_id":1`)
	assert.Contains(t, string(payload), `"board_id":2`)
	assert.Contains(t, string(payload), `"tag_names":["go"]`)
	assert.Contains(t, string(payload), `"deleted_comment_ids":[10,11]`)
}
