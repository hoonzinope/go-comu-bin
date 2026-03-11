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
	post := NewPostChanged("updated", 20, 30, []string{"go", "news"}, nil)
	comment := NewCommentChanged("deleted", 40, 20)
	reaction := NewReactionChanged("set", entity.ReactionTargetPost, 20, 20)
	attachment := NewAttachmentChanged("deleted", 50, 20)

	assert.Equal(t, EventNameBoardChanged, board.EventName())
	assert.Equal(t, EventNamePostChanged, post.EventName())
	assert.Equal(t, EventNameCommentChanged, comment.EventName())
	assert.Equal(t, EventNameReactionChanged, reaction.EventName())
	assert.Equal(t, EventNameAttachmentChanged, attachment.EventName())

	assert.WithinDuration(t, time.Now(), board.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), post.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), comment.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), reaction.OccurredAt(), time.Second)
	assert.WithinDuration(t, time.Now(), attachment.OccurredAt(), time.Second)
}

func TestPostChanged_JSONPayloadContainsMinimalFields(t *testing.T) {
	e := NewPostChanged("deleted", 1, 2, []string{"go"}, []int64{10, 11})

	payload, err := json.Marshal(e)
	assert.NoError(t, err)
	assert.Contains(t, string(payload), `"operation":"deleted"`)
	assert.Contains(t, string(payload), `"post_id":1`)
	assert.Contains(t, string(payload), `"board_id":2`)
	assert.Contains(t, string(payload), `"tag_names":["go"]`)
	assert.Contains(t, string(payload), `"deleted_comment_ids":[10,11]`)
}
