package schemas

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// ExchangeGrant is one thing a client may trade a human's token for: a single
// audience and the most it may ask for there.
type ExchangeGrant struct {
	Audience string   `json:"audience"`
	Scopes   []string `json:"scopes"`
}

// ExchangeGrants is the list of grants a client holds, kept in a jsonb column
// for the same reason StringList is: it is read whole and never joined.
type ExchangeGrants []ExchangeGrant

// Value encodes the list as a JSON array. A nil list is stored as [] rather
// than null, so a reader never has to distinguish the two.
func (grants ExchangeGrants) Value() (driver.Value, error) {
	if grants == nil {
		return "[]", nil
	}
	encoded, err := json.Marshal([]ExchangeGrant(grants))
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

// Scan decodes a jsonb array back into the list.
func (grants *ExchangeGrants) Scan(src any) error {
	switch typed := src.(type) {
	case nil:
		*grants = nil
		return nil
	case []byte:
		return json.Unmarshal(typed, grants)
	case string:
		return json.Unmarshal([]byte(typed), grants)
	default:
		return fmt.Errorf("schemas: cannot scan %T into ExchangeGrants", src)
	}
}

// For returns the grant naming audience.
func (grants ExchangeGrants) For(audience string) (ExchangeGrant, bool) {
	for _, grant := range grants {
		if grant.Audience == audience {
			return grant, true
		}
	}
	return ExchangeGrant{}, false
}
