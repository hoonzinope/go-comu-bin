package model

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type ReactionTargetType string

const (
	ReactionTargetPost    ReactionTargetType = "post"
	ReactionTargetComment ReactionTargetType = "comment"
)

func ParseReactionTargetType(raw string) (ReactionTargetType, bool) {
	switch ReactionTargetType(raw) {
	case ReactionTargetPost, ReactionTargetComment:
		return ReactionTargetType(raw), true
	default:
		return "", false
	}
}

func (t ReactionTargetType) ToEntity() (entity.ReactionTargetType, bool) {
	switch t {
	case ReactionTargetPost:
		return entity.ReactionTargetPost, true
	case ReactionTargetComment:
		return entity.ReactionTargetComment, true
	default:
		return "", false
	}
}

type ReactionType string

const (
	ReactionTypeLike    ReactionType = "like"
	ReactionTypeDislike ReactionType = "dislike"
)

func ParseReactionType(raw string) (ReactionType, bool) {
	switch ReactionType(raw) {
	case ReactionTypeLike, ReactionTypeDislike:
		return ReactionType(raw), true
	default:
		return "", false
	}
}

func (t ReactionType) ToEntity() (entity.ReactionType, bool) {
	switch t {
	case ReactionTypeLike:
		return entity.ReactionTypeLike, true
	case ReactionTypeDislike:
		return entity.ReactionTypeDislike, true
	default:
		return "", false
	}
}

type ReportTargetType string

const (
	ReportTargetPost    ReportTargetType = "post"
	ReportTargetComment ReportTargetType = "comment"
)

func ParseReportTargetType(raw string) (ReportTargetType, bool) {
	switch ReportTargetType(raw) {
	case ReportTargetPost, ReportTargetComment:
		return ReportTargetType(raw), true
	default:
		return "", false
	}
}

func (t ReportTargetType) ToEntity() (entity.ReportTargetType, bool) {
	switch t {
	case ReportTargetPost:
		return entity.ReportTargetPost, true
	case ReportTargetComment:
		return entity.ReportTargetComment, true
	default:
		return "", false
	}
}

type ReportReasonCode string

const (
	ReportReasonSpam     ReportReasonCode = "spam"
	ReportReasonAbuse    ReportReasonCode = "abuse"
	ReportReasonSexual   ReportReasonCode = "sexual"
	ReportReasonViolence ReportReasonCode = "violence"
	ReportReasonIllegal  ReportReasonCode = "illegal"
	ReportReasonOther    ReportReasonCode = "other"
)

func ParseReportReasonCode(raw string) (ReportReasonCode, bool) {
	switch ReportReasonCode(raw) {
	case ReportReasonSpam, ReportReasonAbuse, ReportReasonSexual, ReportReasonViolence, ReportReasonIllegal, ReportReasonOther:
		return ReportReasonCode(raw), true
	default:
		return "", false
	}
}

func (c ReportReasonCode) ToEntity() (entity.ReportReasonCode, bool) {
	switch c {
	case ReportReasonSpam:
		return entity.ReportReasonSpam, true
	case ReportReasonAbuse:
		return entity.ReportReasonAbuse, true
	case ReportReasonSexual:
		return entity.ReportReasonSexual, true
	case ReportReasonViolence:
		return entity.ReportReasonViolence, true
	case ReportReasonIllegal:
		return entity.ReportReasonIllegal, true
	case ReportReasonOther:
		return entity.ReportReasonOther, true
	default:
		return "", false
	}
}

type ReportStatus string

const (
	ReportStatusPending  ReportStatus = "pending"
	ReportStatusAccepted ReportStatus = "accepted"
	ReportStatusRejected ReportStatus = "rejected"
)

func ParseReportStatus(raw string) (ReportStatus, bool) {
	switch ReportStatus(raw) {
	case ReportStatusPending, ReportStatusAccepted, ReportStatusRejected:
		return ReportStatus(raw), true
	default:
		return "", false
	}
}

func (s ReportStatus) ToEntity() (entity.ReportStatus, bool) {
	switch s {
	case ReportStatusPending:
		return entity.ReportStatusPending, true
	case ReportStatusAccepted:
		return entity.ReportStatusAccepted, true
	case ReportStatusRejected:
		return entity.ReportStatusRejected, true
	default:
		return "", false
	}
}

type SuspensionDuration string

const (
	SuspensionDuration7Days     SuspensionDuration = "7d"
	SuspensionDuration15Days    SuspensionDuration = "15d"
	SuspensionDuration30Days    SuspensionDuration = "30d"
	SuspensionDurationUnlimited SuspensionDuration = "unlimited"
)

func ParseSuspensionDuration(raw string) (SuspensionDuration, bool) {
	switch SuspensionDuration(raw) {
	case SuspensionDuration7Days, SuspensionDuration15Days, SuspensionDuration30Days, SuspensionDurationUnlimited:
		return SuspensionDuration(raw), true
	default:
		return "", false
	}
}

func (d SuspensionDuration) ToEntity() (entity.SuspensionDuration, bool) {
	switch d {
	case SuspensionDuration7Days:
		return entity.SuspensionDuration7Days, true
	case SuspensionDuration15Days:
		return entity.SuspensionDuration15Days, true
	case SuspensionDuration30Days:
		return entity.SuspensionDuration30Days, true
	case SuspensionDurationUnlimited:
		return entity.SuspensionDurationUnlimited, true
	default:
		return "", false
	}
}
