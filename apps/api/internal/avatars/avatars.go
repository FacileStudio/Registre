// Package avatars owns profile pictures: normalising an upload, storing it,
// and naming the URL apps fetch it from.
//
// Apps never receive the bytes from registre directly. The `picture` claim
// carries a public URL, and porte fetches it at every sign-in into the app's
// own store, which is how one upload reaches every app in the suite without a
// porte change and without registre knowing which apps exist.
package avatars

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// Path is the public prefix pictures are served under.
const Path = "/avatars/"

// Service stores pictures and names their public URLs under issuer.
type Service struct {
	db     *gorm.DB
	issuer string
}

// New builds the service over db, naming URLs under issuer.
func New(db *gorm.DB, issuer string) *Service {
	return &Service{db: db, issuer: strings.TrimSuffix(issuer, "/")}
}

// Put normalises upload and makes it userID's picture, replacing any earlier
// one, and returns the new picture's URL. The old row is deleted in the same
// transaction, so a user never has two pictures and never has none mid-swap.
func (s *Service) Put(ctx context.Context, userID string, upload []byte) (string, error) {
	data, err := Normalize(upload)
	if err != nil {
		return "", err
	}
	id, err := ids.New()
	if err != nil {
		return "", fmt.Errorf("avatars: mint an id: %w", err)
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&schemas.Avatar{}).Error; err != nil {
			return err
		}
		return tx.Create(&schemas.Avatar{ID: id, UserID: userID, Data: data}).Error
	})
	if err != nil {
		return "", fmt.Errorf("avatars: store the picture: %w", err)
	}
	return s.url(id), nil
}

// Remove deletes userID's picture. Removing a picture that does not exist is
// not an error: the state the caller asked for is the state they are in.
func (s *Service) Remove(ctx context.Context, userID string) error {
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&schemas.Avatar{}).Error
	if err != nil {
		return fmt.Errorf("avatars: remove the picture: %w", err)
	}
	return nil
}

// Picture returns userID's picture URL, or "" when they have none. It reads
// the id alone, never the bytes, because it runs on every token issuance.
func (s *Service) Picture(ctx context.Context, userID string) (string, error) {
	var row schemas.Avatar
	err := s.db.WithContext(ctx).Select("id").Where("user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("avatars: read the picture id: %w", err)
	}
	return s.url(row.ID), nil
}

// ByID reads one stored picture. A missing id answers gorm.ErrRecordNotFound.
func (s *Service) ByID(ctx context.Context, id string) (schemas.Avatar, error) {
	var row schemas.Avatar
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	return row, err
}

// url is the absolute address of picture id. Absolute because porte refuses
// anything that is not https, and a relative claim has no scheme at all.
func (s *Service) url(id string) string { return s.issuer + Path + id }
