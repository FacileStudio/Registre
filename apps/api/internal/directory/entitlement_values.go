package directory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// MaxEntitlementKeys bounds how many features one scope may carry. The suite
// has six; two hundred leaves room without letting a push grow a row
// without limit.
const MaxEntitlementKeys = 200

// MaxScopes bounds how many spaces and people one push may carry together.
// The 1 MiB body cap alone would admit about a hundred thousand.
const MaxScopes = 10_000

// MaxRevisionJump bounds how far one push may move the revision. Revisions
// only ever rise, so a push far ahead would make every honest push after it
// stale.
const MaxRevisionJump = 1_000_000

// entitlementKey is the app.feature shape every key takes, such as
// nuage.storage_gb.
var entitlementKey = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

// entitlementRow is one stored scope as the snapshot query reads it.
type entitlementRow struct {
	ID     string
	Values string
}

// decode turns the stored JSON object back into the wire map, answering nil
// for an empty or unreadable one so the scope falls back to the defaults.
func (r entitlementRow) decode() map[string]json.RawMessage {
	var out map[string]json.RawMessage
	if json.Unmarshal([]byte(r.Values), &out) != nil || len(out) == 0 {
		return nil
	}
	return out
}

// validatePush checks a whole push before anything is written.
func validatePush(revision int64, spaces []SpaceEntitlements, people []PersonEntitlements) error {
	if revision < 1 {
		return fmt.Errorf("%w: revision must be at least 1", ErrInvalidEntitlements)
	}
	if len(spaces)+len(people) > MaxScopes {
		return fmt.Errorf("%w: more than %d scopes in one push", ErrInvalidEntitlements, MaxScopes)
	}
	for _, space := range spaces {
		if space.ID == "" {
			return fmt.Errorf("%w: a space needs an id", ErrInvalidEntitlements)
		}
		if err := validateValues(space.Entitlements); err != nil {
			return err
		}
	}
	return validatePeople(people)
}

// validatePeople checks every pushed person names somebody and carries
// valid values.
func validatePeople(people []PersonEntitlements) error {
	for _, person := range people {
		if person.Subject == "" && person.Email == "" {
			return fmt.Errorf("%w: a person needs a sub or an email", ErrInvalidEntitlements)
		}
		if err := validateValues(person.Entitlements); err != nil {
			return err
		}
	}
	return nil
}

// validateValues checks one scope: bounded size, app.feature keys, and each
// value a JSON bool or an integer of at least -1, -1 meaning unlimited.
func validateValues(values map[string]json.RawMessage) error {
	if len(values) > MaxEntitlementKeys {
		return fmt.Errorf("%w: more than %d keys in one scope", ErrInvalidEntitlements, MaxEntitlementKeys)
	}
	for key, raw := range values {
		if !entitlementKey.MatchString(key) {
			return fmt.Errorf("%w: key %q is not app.feature", ErrInvalidEntitlements, key)
		}
		if !validValue(raw) {
			return fmt.Errorf("%w: %s must be a bool or an integer of at least -1", ErrInvalidEntitlements, key)
		}
	}
	return nil
}

// validValue reports whether raw is true, false, or an integer >= -1 that
// fits an int64.
func validValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if string(trimmed) == "true" || string(trimmed) == "false" {
		return true
	}
	number, err := strconv.ParseInt(string(trimmed), 10, 64)
	return err == nil && number >= -1
}
