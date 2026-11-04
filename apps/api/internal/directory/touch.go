package directory

import (
	"context"

	"gorm.io/gorm"
)

// Touch bumps the directory version and nudges the apps without writing
// anything else. Erasing an account removes its memberships by cascade, and
// the apps must pull a snapshot that no longer lists the person.
func (s *Service) Touch(ctx context.Context) error {
	return s.commit(ctx, func(*gorm.DB) error { return nil })
}
