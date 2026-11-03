package devices

import (
	"context"
	"errors"
	"fmt"
	"github.com/FacileStudio/Registre/apps/api/internal/access"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// The half a human drives. zitadel/oidc defines no way to complete or refuse a
// device flow — the library polls a state and leaves the deciding entirely to
// the application — so these three are registre's own contract, and
// modules/device is what puts them behind HTTP.

// Pending returns the flow a user code names, so the verification page can say
// what is actually being approved before anybody approves it.
//
// A flow that is expired, already answered or already spent is ErrUnknownCode,
// the same as one that never existed. Showing "that code was already used"
// would confirm to a guesser that they had found a real one.
func (s *Service) Pending(ctx context.Context, userCode string) (schemas.DeviceCode, error) {
	var row schemas.DeviceCode
	err := s.db.WithContext(ctx).
		Where("user_code = ?", Normalise(userCode)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.DeviceCode{}, ErrUnknownCode
	}
	if err != nil {
		return schemas.DeviceCode{}, fmt.Errorf("devices: read the flow: %w", err)
	}
	if row.Approved() || row.Denied() || !row.ExpiresAt.After(s.now()) {
		return schemas.DeviceCode{}, ErrUnknownCode
	}
	return row, nil
}

// Approve records that a human said yes, and to whom the resulting token
// belongs.
//
// The subject, the authentication method and the authentication time are
// copied out of the session the human is already holding with registre rather
// than invented here. A token minted from this flow therefore says the same
// things about how its owner signed in as one minted from a browser login,
// which is what lets a relying party treat the two identically. A human who
// may not use the requesting app is refused with access.ErrNoAccess.
func (s *Service) Approve(ctx context.Context, userCode string, by Approval) error {
	var clientID string
	if err := s.db.WithContext(ctx).Model(&schemas.DeviceCode{}).Select("client_id").
		Where("user_code = ?", Normalise(userCode)).Scan(&clientID).Error; err != nil {
		return fmt.Errorf("devices: read the flow's app: %w", err)
	}
	if clientID != "" {
		if err := access.New(s.db, "").Require(ctx, clientID, by.UserID); err != nil {
			return err
		}
	}
	return s.answer(ctx, userCode, map[string]any{
		"approved_at": s.now(),
		"user_id":     by.UserID,
		"amr":         schemas.StringList(by.AMR),
		"auth_time":   by.AuthTime,
	})
}

// Deny records that a human said no. The flow is answered rather than deleted,
// so the CLI's next poll gets access_denied and stops, instead of polling a
// missing row until it times out.
func (s *Service) Deny(ctx context.Context, userCode string) error {
	return s.answer(ctx, userCode, map[string]any{"denied_at": s.now()})
}

// answer writes one decision onto an unanswered, unexpired flow.
//
// The conditions are in the WHERE clause rather than in a read before the
// write because two tabs on the same code is an ordinary thing for a human to
// do, and a read-then-write would let the second one overwrite the first. The
// database decides who was first; whoever loses is told the code is unknown,
// which is also what they would be told if they had made it up.
func (s *Service) answer(ctx context.Context, userCode string, fields map[string]any) error {
	result := s.db.WithContext(ctx).Model(&schemas.DeviceCode{}).
		Where(
			"user_code = ? AND approved_at IS NULL AND denied_at IS NULL AND expires_at > ?",
			Normalise(userCode), s.now(),
		).
		Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("devices: answer the flow: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrUnknownCode
	}
	return nil
}
