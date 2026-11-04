package gdpr

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// SoleOwnerError refuses a deletion request from the only owner of spaces
// that other people still use: deleting them would leave those spaces with
// nobody able to manage them. The person transfers ownership first.
type SoleOwnerError struct {
	Spaces []string
}

// Error names the spaces.
func (e *SoleOwnerError) Error() string {
	return "gdpr: the only owner of " + strings.Join(e.Spaces, ", ")
}

// soleOwnerQuery selects the live spaces userID alone owns that have other
// members.
const soleOwnerQuery = `SELECT s.id, s.name FROM spaces s
	JOIN space_members m ON m.space_id = s.id AND m.user_id = ? AND m.role = 'owner'
	WHERE s.deleted_at IS NULL
	  AND (SELECT count(*) FROM space_members o WHERE o.space_id = s.id AND o.role = 'owner') = 1
	  AND (SELECT count(*) FROM space_members x WHERE x.space_id = s.id) > 1
	ORDER BY s.name`

// ownedSpace is one row of soleOwnerQuery.
type ownedSpace struct {
	ID   string
	Name string
}

// soleOwned answers the spaces userID is the only owner of while others use them.
func soleOwned(ctx context.Context, db *gorm.DB, userID string) ([]ownedSpace, error) {
	var rows []ownedSpace
	if err := db.WithContext(ctx).Raw(soleOwnerQuery, userID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("gdpr: read owned spaces: %w", err)
	}
	return rows, nil
}

// refuseSoleOwner answers a SoleOwnerError when userID still alone owns a
// space others use.
func refuseSoleOwner(ctx context.Context, db *gorm.DB, userID string) error {
	rows, err := soleOwned(ctx, db, userID)
	if err != nil || len(rows) == 0 {
		return err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return &SoleOwnerError{Spaces: names}
}

// handOver promotes, in every space userID alone owns, the most senior other
// member to owner: an admin first, then whoever joined earliest. It runs at
// erasure, when the person became sole owner again during the grace period;
// erasure is a right and is not withheld for it.
func handOver(tx *gorm.DB, userID string) error {
	rows, err := soleOwned(context.Background(), tx, userID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		err := tx.Exec(`UPDATE space_members SET role = 'owner'
			WHERE space_id = ? AND user_id = (
				SELECT user_id FROM space_members WHERE space_id = ? AND user_id <> ?
				ORDER BY CASE role WHEN 'admin' THEN 0 ELSE 1 END, created_at, user_id LIMIT 1)`,
			row.ID, row.ID, userID).Error
		if err != nil {
			return fmt.Errorf("gdpr: hand over %s: %w", row.ID, err)
		}
	}
	return nil
}
