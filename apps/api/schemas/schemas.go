// Package schemas holds the GORM models registre queries through.
//
// The schema itself is owned by migrations/, not by these structs: AutoMigrate
// is unordered, has no down path and silently no-ops on a rename. A model here
// describes a table that a migration already created, and drifting from it is
// a bug in one of the two.
package schemas

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList is a short list of strings held in a jsonb column.
//
// Scopes, audiences and grant types are lists that are read whole and never
// joined against, so a jsonb array costs one column and no extra table. It is
// jsonb rather than text[] because the pgx driver hands GORM a Postgres array
// as an opaque string, which every call site would then have to parse.
type StringList []string

// Value encodes the list as a JSON array. A nil list is stored as [] rather
// than null, so a reader never has to distinguish the two.
func (list StringList) Value() (driver.Value, error) {
	if list == nil {
		return "[]", nil
	}
	encoded, err := json.Marshal([]string(list))
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

// Scan decodes a jsonb array back into the list.
func (list *StringList) Scan(src any) error {
	switch typed := src.(type) {
	case nil:
		*list = nil
		return nil
	case []byte:
		return json.Unmarshal(typed, list)
	case string:
		return json.Unmarshal([]byte(typed), list)
	default:
		return fmt.Errorf("schemas: cannot scan %T into a StringList", src)
	}
}

// Contains reports whether value is in the list.
func (list StringList) Contains(value string) bool {
	for _, candidate := range list {
		if candidate == value {
			return true
		}
	}
	return false
}
