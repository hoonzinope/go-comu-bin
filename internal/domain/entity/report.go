package entity

import "time"

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

type Report struct {
	ID             int64
	TargetType     ReportTargetType
	TargetID       int64
	ReporterUserID int64
	ReasonCode     ReportReasonCode
	ReasonDetail   string
	Status         ReportStatus
	ResolutionNote string
	ResolvedBy     *int64
	ResolvedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewReport(targetType ReportTargetType, targetID, reporterUserID int64, reasonCode ReportReasonCode, reasonDetail string) *Report {
	now := time.Now()
	return &Report{
		TargetType:     targetType,
		TargetID:       targetID,
		ReporterUserID: reporterUserID,
		ReasonCode:     reasonCode,
		ReasonDetail:   reasonDetail,
		Status:         ReportStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (r *Report) Resolve(status ReportStatus, resolutionNote string, resolverID int64) bool {
	if status != ReportStatusAccepted && status != ReportStatusRejected {
		return false
	}
	now := time.Now()
	r.Status = status
	r.ResolutionNote = resolutionNote
	r.ResolvedBy = &resolverID
	r.ResolvedAt = &now
	r.UpdatedAt = now
	return true
}

