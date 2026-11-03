package schemas

import "time"

// Avatar is one human's profile picture. ID is random and changes on every
// upload, because it is the URL apps fetch and a new picture must be a new
// URL. Data is always a PNG registre encoded itself.
type Avatar struct {
	ID        string `gorm:"primaryKey"`
	UserID    string
	Data      []byte
	CreatedAt time.Time
}

// TableName is the table this model reads.
func (Avatar) TableName() string { return "avatars" }
