package schemas

import "time"

// Space is one shared space, the same team in every app of the suite. ID is
// random and stable across renames; apps store it to link their own row.
type Space struct {
	ID   string `gorm:"primaryKey"`
	Name string

	// DeletedAt marks a space an owner deleted. It stays in the snapshot
	// with no members so every app empties its copy; see migration 00011.
	DeletedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName is the table this model reads.
func (Space) TableName() string { return "spaces" }

// SpaceMember is one human's rank in one space, on the suite ladder:
// owner > admin > member.
type SpaceMember struct {
	SpaceID   string `gorm:"primaryKey"`
	UserID    string `gorm:"primaryKey"`
	Role      string
	CreatedAt time.Time
}

// TableName is the table this model reads.
func (SpaceMember) TableName() string { return "space_members" }

// SpaceInvitation is a pending offer of a role in a space, addressed to an
// email. Accepting it creates the SpaceMember row; until then the person is
// in no snapshot.
type SpaceInvitation struct {
	ID        string `gorm:"primaryKey"`
	SpaceID   string
	Email     string
	Role      string
	InvitedBy string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// TableName is the table this model reads.
func (SpaceInvitation) TableName() string { return "space_invitations" }
