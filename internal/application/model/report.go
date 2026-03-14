package model

import "time"

type Report struct {
	ID             int64
	TargetType     string
	TargetID       int64
	ReporterUserID int64
	ReporterUUID   string
	ReasonCode     string
	ReasonDetail   string
	Status         string
	ResolutionNote string
	ResolvedBy     *int64
	ResolvedByUUID *string
	ResolvedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ReportList struct {
	Reports    []Report
	Limit      int
	LastID     int64
	HasMore    bool
	NextLastID *int64
}

