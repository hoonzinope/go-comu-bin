package post

import (
	"sort"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postTagCoordinator struct {
	tagRepository     port.TagRepository
	postTagRepository port.PostTagRepository
}

type TagCoordinator = postTagCoordinator

func newPostTagCoordinator(tagRepository port.TagRepository, postTagRepository port.PostTagRepository) *postTagCoordinator {
	return &postTagCoordinator{tagRepository: tagRepository, postTagRepository: postTagRepository}
}

func NewTagCoordinator(tagRepository port.TagRepository, postTagRepository port.PostTagRepository) *TagCoordinator {
	return newPostTagCoordinator(tagRepository, postTagRepository)
}

func (c *postTagCoordinator) activeTagNamesByPostIDTx(tx port.TxScope, postID int64) ([]string, error) {
	tags, err := c.tagsForPostTx(tx, postID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names, nil
}

func (c *postTagCoordinator) ActiveTagNamesByPostIDTx(tx port.TxScope, postID int64) ([]string, error) {
	return c.activeTagNamesByPostIDTx(tx, postID)
}

func (c *postTagCoordinator) tagsForPostTx(tx port.TxScope, postID int64) ([]model.Tag, error) {
	txCtx := tx.Context()
	relations, err := tx.PostTagRepository().SelectActiveByPostID(txCtx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select active tags by post id", err)
	}
	if len(relations) == 0 {
		return []model.Tag{}, nil
	}
	tagIDs := make([]int64, 0, len(relations))
	for _, relation := range relations {
		tagIDs = append(tagIDs, relation.TagID)
	}
	tags, err := tx.TagRepository().SelectByIDs(txCtx, tagIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select tags by ids", err)
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})
	return mapper.TagsFromEntities(tags), nil
}

func (c *postTagCoordinator) TagsForPostTx(tx port.TxScope, postID int64) ([]model.Tag, error) {
	return c.tagsForPostTx(tx, postID)
}

func (c *postTagCoordinator) syncPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	txCtx := tx.Context()
	currentRelations, err := tx.PostTagRepository().SelectActiveByPostID(txCtx, postID)
	if err != nil {
		return customerror.WrapRepository("select active post tags for sync", err)
	}

	targetTagIDs := make(map[int64]struct{}, len(normalizedTags))
	for _, tagName := range normalizedTags {
		tagID, resolveErr := c.getOrCreateTagID(tx, tagName)
		if resolveErr != nil {
			return resolveErr
		}
		targetTagIDs[tagID] = struct{}{}
		if upsertErr := tx.PostTagRepository().UpsertActive(txCtx, postID, tagID); upsertErr != nil {
			return customerror.WrapRepository("upsert active post tag", upsertErr)
		}
	}
	for _, relation := range currentRelations {
		if _, ok := targetTagIDs[relation.TagID]; ok {
			continue
		}
		if deleteErr := tx.PostTagRepository().SoftDelete(txCtx, postID, relation.TagID); deleteErr != nil {
			return customerror.WrapRepository("soft delete post tag", deleteErr)
		}
	}
	return nil
}

func (c *postTagCoordinator) SyncPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	return c.syncPostTags(tx, postID, normalizedTags)
}

func (c *postTagCoordinator) upsertPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	for _, tagName := range normalizedTags {
		tagID, err := c.getOrCreateTagID(tx, tagName)
		if err != nil {
			return err
		}
		if err := tx.PostTagRepository().UpsertActive(tx.Context(), postID, tagID); err != nil {
			return customerror.WrapRepository("upsert post tag relation", err)
		}
	}
	return nil
}

func (c *postTagCoordinator) UpsertPostTags(tx port.TxScope, postID int64, normalizedTags []string) error {
	return c.upsertPostTags(tx, postID, normalizedTags)
}

func (c *postTagCoordinator) getOrCreateTagID(tx port.TxScope, tagName string) (int64, error) {
	tag, err := tx.TagRepository().SelectByName(tx.Context(), tagName)
	if err != nil {
		return 0, customerror.WrapRepository("select tag by name", err)
	}
	if tag != nil {
		return tag.ID, nil
	}
	tagID, err := tx.TagRepository().Save(tx.Context(), entity.NewTag(tagName))
	if err != nil {
		return 0, customerror.WrapRepository("save tag", err)
	}
	return tagID, nil
}

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > maxPostTags {
		return nil, customerror.ErrInvalidInput
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := normalizeTagName(tag)
		if normalized == "" || len(normalized) > maxTagLength {
			return nil, customerror.ErrInvalidInput
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) > maxPostTags {
		return nil, customerror.ErrInvalidInput
	}
	sort.Strings(out)
	return out, nil
}

func normalizeTagName(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func unionTagNames(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	out := make([]string, 0, len(left)+len(right))
	for _, item := range append(append([]string{}, left...), right...) {
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
