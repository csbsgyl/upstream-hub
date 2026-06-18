package storage

import (
	"time"

	"gorm.io/gorm"
)

type AuditLogs struct{ db *gorm.DB }

func NewAuditLogs(db *gorm.DB) *AuditLogs { return &AuditLogs{db: db} }

func (r *AuditLogs) Append(l *AuditLog) error {
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	return r.db.Create(l).Error
}

func (r *AuditLogs) List(limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}
	var list []AuditLog
	if err := r.db.Order("created_at DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
