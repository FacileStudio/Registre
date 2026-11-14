package schemas

import "time"

// WaitlistEntry is one address waiting to hear that a product shipped. Email
// is stored lowercased and trimmed, and an address appears once per product.
type WaitlistEntry struct {
	ID             string `gorm:"primaryKey"`
	Email          string
	Product        string
	Locale         string
	ConsentVersion string
	CreatedAt      time.Time
}

// TableName is the table this model reads.
func (WaitlistEntry) TableName() string { return "waitlist_entries" }
