package service

import (
	"errors"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.ReactionUseCase = (*ReactionService)(nil)

type ReactionService struct {
	userRepository     port.UserRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	unitOfWork         port.UnitOfWork
	cache              port.Cache
	eventPublisher     port.EventPublisher
	cachePolicy        appcache.Policy
	logger             port.Logger
}

func NewReactionService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, logger ...port.Logger) *ReactionService {
	return NewReactionServiceWithPublisher(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy, logger...)
}

func NewReactionServiceWithPublisher(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, unitOfWork port.UnitOfWork, cache port.Cache, eventPublisher port.EventPublisher, cachePolicy appcache.Policy, logger ...port.Logger) *ReactionService {
	return &ReactionService{
		userRepository:     userRepository,
		postRepository:     postRepository,
		commentRepository:  commentRepository,
		reactionRepository: reactionRepository,
		unitOfWork:         unitOfWork,
		cache:              cache,
		eventPublisher:     resolveEventPublisher(eventPublisher),
		cachePolicy:        cachePolicy,
		logger:             resolveLogger(logger),
	}
}

func (s *ReactionService) SetReaction(UserID, TargetID int64, TargetType entity.ReactionTargetType, ReactionType entity.ReactionType) (bool, error) {
	created, changed, err := s.withReactionTransaction(UserID, TargetID, TargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		_, created, changed, err := tx.ReactionRepository().SetUserTargetReaction(UserID, TargetID, TargetType, ReactionType)
		if err != nil {
			return false, false, customError.WrapRepository("set user target reaction", err)
		}
		if (created || changed) && detailPostID != nil {
			if err := appendEventsToOutbox(tx, appevent.NewReactionChanged("set", TargetType, TargetID, *detailPostID)); err != nil {
				return false, false, err
			}
		}
		return created, changed, nil
	})
	if err != nil {
		return false, err
	}
	if !created && !changed {
		return false, nil
	}
	return created, nil
}

func (s *ReactionService) DeleteReaction(UserID, TargetID int64, TargetType entity.ReactionTargetType) error {
	deleted, _, err := s.withReactionTransaction(UserID, TargetID, TargetType, func(tx port.TxScope, detailPostID *int64) (bool, bool, error) {
		deleted, err := tx.ReactionRepository().DeleteUserTargetReaction(UserID, TargetID, TargetType)
		if err != nil {
			return false, false, customError.WrapRepository("delete user target reaction", err)
		}
		if deleted && detailPostID != nil {
			if err := appendEventsToOutbox(tx, appevent.NewReactionChanged("unset", TargetType, TargetID, *detailPostID)); err != nil {
				return false, false, err
			}
		}
		return deleted, deleted, nil
	})
	if err != nil {
		return err
	}
	if !deleted {
		return nil
	}
	return nil
}

func (s *ReactionService) GetReactionsByTarget(targetID int64, targetType entity.ReactionTargetType) ([]model.Reaction, error) {
	cacheKey := key.ReactionList(string(targetType), targetID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		if err := s.ensureTargetExists(targetID, targetType); err != nil {
			return nil, err
		}
		reactions, err := s.reactionRepository.GetByTarget(targetID, targetType)
		if err != nil {
			return nil, customError.WrapRepository("select reactions by target", err)
		}
		reactionModels, err := s.reactionsFromEntities(reactions)
		if err != nil {
			return nil, err
		}
		return reactionModels, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load reaction list cache", err)
	}
	reactions, ok := value.([]model.Reaction)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode reaction list cache payload")
	}
	return reactions, nil
}

func (s *ReactionService) withReactionTransaction(userID, targetID int64, targetType entity.ReactionTargetType, mutate func(tx port.TxScope, detailPostID *int64) (bool, bool, error)) (bool, bool, error) {
	var created bool
	var changed bool
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(userID)
		if err != nil {
			return customError.WrapRepository("select user by id for reaction", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		postID, err := s.ensureTargetExistsTx(tx, targetID, targetType)
		if err != nil {
			return err
		}
		created, changed, err = mutate(tx, postID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return false, false, err
	}
	return created, changed, nil
}

func (s *ReactionService) reactionsFromEntities(reactions []*entity.Reaction) ([]model.Reaction, error) {
	userUUIDs, err := userUUIDsForReactions(s.userRepository, reactions)
	if err != nil {
		return nil, err
	}
	out := make([]model.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		userUUID, ok := userUUIDs[reaction.UserID]
		if !ok {
			return nil, customError.WrapRepository("select users by ids including deleted", errors.New("reaction user not found"))
		}
		reactionModel := mapper.ReactionFromEntity(reaction)
		reactionModel.UserUUID = userUUID
		out = append(out, reactionModel)
	}
	return out, nil
}

func (s *ReactionService) ensureTargetExistsTx(tx port.TxScope, targetID int64, targetType entity.ReactionTargetType) (*int64, error) {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := tx.PostRepository().SelectPostByID(targetID)
		if err != nil {
			return nil, customError.WrapRepository("select post by id for ensure reaction target", err)
		}
		if post == nil {
			return nil, customError.ErrPostNotFound
		}
		postID := post.ID
		return &postID, nil
	case entity.ReactionTargetComment:
		comment, err := tx.CommentRepository().SelectCommentByID(targetID)
		if err != nil {
			return nil, customError.WrapRepository("select comment by id for ensure reaction target", err)
		}
		if comment == nil {
			return nil, customError.ErrCommentNotFound
		}
		postID := comment.PostID
		return &postID, nil
	default:
		return nil, customError.ErrInternalServerError
	}
}

func (s *ReactionService) ensureTargetExists(targetID int64, targetType entity.ReactionTargetType) error {
	switch targetType {
	case entity.ReactionTargetPost:
		post, err := s.postRepository.SelectPostByID(targetID)
		if err != nil {
			return customError.WrapRepository("select post by id for ensure reaction target", err)
		}
		if post == nil {
			return customError.ErrPostNotFound
		}
		return nil
	case entity.ReactionTargetComment:
		comment, err := s.commentRepository.SelectCommentByID(targetID)
		if err != nil {
			return customError.WrapRepository("select comment by id for ensure reaction target", err)
		}
		if comment == nil {
			return customError.ErrCommentNotFound
		}
		return nil
	default:
		return customError.ErrInternalServerError
	}
}
