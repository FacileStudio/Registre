package schemas

import "time"

// AppGrant lets one user use one application, with the roles they hold in
// it. Under AccessAssigned the row is what admits them; under AccessEveryone
// it only carries roles. What reaches a token is Roles intersected with the
// client's VisibleRoles, never the raw list (SPEC §6).
type AppGrant struct {
	ClientID  string     `gorm:"primaryKey"`
	UserID    string     `gorm:"primaryKey"`
	Roles     StringList `gorm:"type:jsonb"`
	GrantedAt time.Time
	GrantedBy string
}

// TableName pins the table the migration creates.
func (AppGrant) TableName() string { return "app_grants" }

// AdminEvent records one change an operator made through the dashboard.
type AdminEvent struct {
	ID         int64 `gorm:"primaryKey"`
	At         time.Time
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	Detail     JSONMap `gorm:"type:jsonb"`
}

// TableName pins the table the migration creates.
func (AdminEvent) TableName() string { return "admin_events" }
