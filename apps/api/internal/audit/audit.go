// Package audit records what operators change through the dashboard, so who
// granted, revoked, disabled or rotated what is answerable after the fact.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
	"gorm.io/gorm"
)

// Service writes and reads admin events.
type Service struct {
	db *gorm.DB
}

// New builds the audit service.
func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Target names what an event changed.
type Target struct {
	Type string
	ID   string
}

// Record stores one event. It is called after the change succeeded, so the
// log never claims something that did not happen.
func (s *Service) Record(ctx context.Context, actorID, action string, target Target, detail map[string]any) error {
	return Write(s.db.WithContext(ctx), actorID, action, target, detail)
}

// Write stores one event through db, which may be a transaction: a change
// that must never be applied without its record writes both in one.
func Write(db *gorm.DB, actorID, action string, target Target, detail map[string]any) error {
	row := schemas.AdminEvent{
		At: time.Now().UTC(), ActorID: actorID, Action: action,
		TargetType: target.Type, TargetID: target.ID, Detail: schemas.JSONMap(detail),
	}
	if err := db.Create(&row).Error; err != nil {
		return fmt.Errorf("audit: record %s: %w", action, err)
	}
	return nil
}

// List answers up to limit events older than before (all when before is 0),
// newest first.
func (s *Service) List(ctx context.Context, before int64, limit int) ([]schemas.AdminEvent, error) {
	query := s.db.WithContext(ctx).Order("at DESC, id DESC").Limit(limit)
	if before > 0 {
		query = query.Where("id < ?", before)
	}
	var rows []schemas.AdminEvent
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("audit: list events: %w", err)
	}
	return rows, nil
}
