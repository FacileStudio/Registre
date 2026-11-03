// Package avatars owns profile pictures: normalising an upload, storing it,
// and naming the URL apps fetch it from.
//
// The `picture` claim carries one stable URL per human, <issuer>/avatars/<sub>,
// which always serves the current picture. Apps store that URL and render it,
// so a new upload shows in every app on its next page load: nothing is pushed,
// nothing is re-synced, and registre never needs to know which apps exist.
// The response revalidates on every load (no-cache plus an ETag), which is
// what makes the stable URL safe to cache at all.
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
	return s.versioned(userID, id), nil
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

// URL is userID's stable picture URL, the one the `picture` claim carries.
//
// It is emitted for every human, picture or not, and answers 404 until one is
// uploaded. That is what makes a first upload instant: an app stores the URL
// at its first sign-in and never needs another to learn that a picture now
// exists behind it. An app renders initials on the 404.
func (s *Service) URL(userID string) string { return s.url(userID) }

// Versioned returns userID's picture URL with the current upload's id as a
// query, or "" when they have none. It is what registre's own pages render:
// the stable URL does not change on upload, so an <img> already showing it
// would never ask again.
func (s *Service) Versioned(ctx context.Context, userID string) (string, error) {
	id, err := s.current(ctx, userID)
	if id == "" || err != nil {
		return "", err
	}
	return s.versioned(userID, id), nil
}

// Lookup reads the picture a URL key names: a user id, which is what the
// claim carries, or an upload id, which older claims carried. A miss answers
// gorm.ErrRecordNotFound.
func (s *Service) Lookup(ctx context.Context, key string) (schemas.Avatar, error) {
	var row schemas.Avatar
	err := s.db.WithContext(ctx).Where("user_id = ? OR id = ?", key, key).Take(&row).Error
	return row, err
}

// current is the id of userID's picture, or "" when they have none.
func (s *Service) current(ctx context.Context, userID string) (string, error) {
	var row schemas.Avatar
	err := s.db.WithContext(ctx).Select("id").Where("user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("avatars: read the picture id: %w", err)
	}
	return row.ID, nil
}

// url is the absolute, stable address of userID's picture. Absolute because
// porte refuses anything that is not https, and a relative claim has no
// scheme at all.
func (s *Service) url(userID string) string { return s.issuer + Path + userID }

// versioned is url with the upload id appended, so it changes on upload.
func (s *Service) versioned(userID, id string) string { return s.url(userID) + "?v=" + id }
