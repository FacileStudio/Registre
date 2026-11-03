package schemas

import "time"

// Space is one shared space, the same team in every app of the suite. ID is
// random and stable across renames; apps store it to link their own row.
type Space struct {
	ID        string `gorm:"primaryKey"`
	Name      string
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
