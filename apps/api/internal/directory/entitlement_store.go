package directory

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// writeBatch is how many scopes one INSERT or lookup carries. It keeps every
// statement well under Postgres's 65535 bind parameters.
const writeBatch = 500

// scopeSet is pushed values keyed by space id or user id, in first-seen
// order, the last value winning when a push repeats a key.
type scopeSet struct {
	keys   []string
	values map[string]string
}

// add records values for key.
func (set *scopeSet) add(key string, values map[string]json.RawMessage) {
	encoded, _ := json.Marshal(values)
	if _, seen := set.values[key]; !seen {
		set.keys = append(set.keys, key)
	}
	set.values[key] = string(encoded)
}

// replaceSpaces clears every space's values and writes the pushed ones in
// batches, skipping ids that name no space.
func replaceSpaces(tx *gorm.DB, in []SpaceEntitlements) (int, int, error) {
	if err := tx.Exec("DELETE FROM space_entitlements").Error; err != nil {
		return 0, 0, err
	}
	set := scopeSet{values: map[string]string{}}
	for _, space := range in {
		set.add(space.ID, space.Entitlements)
	}
	stored, err := insertScopes(tx, `INSERT INTO space_entitlements (space_id, "values")
		SELECT v.id, v.vals::jsonb FROM (VALUES %s) AS v(id, vals) JOIN spaces s ON s.id = v.id`, set)
	return stored, len(set.keys) - stored, err
}

// replacePeople clears every person's values and writes the pushed ones in
// batches, skipping people Registre cannot resolve.
func replacePeople(tx *gorm.DB, in []PersonEntitlements) (int, int, error) {
	if err := tx.Exec("DELETE FROM user_entitlements").Error; err != nil {
		return 0, 0, err
	}
	bySubject, byEmail, err := resolvePeople(tx, in)
	if err != nil {
		return 0, 0, err
	}
	set, skipped := scopeSet{values: map[string]string{}}, 0
	for _, person := range in {
		userID := bySubject[person.Subject]
		if userID == "" {
			userID = byEmail[normalEmail(person.Email)]
		}
		if userID == "" {
			skipped++
			continue
		}
		set.add(userID, person.Entitlements)
	}
	stored, err := insertScopes(tx, `INSERT INTO user_entitlements (user_id, "values")
		SELECT v.id, v.vals::jsonb FROM (VALUES %s) AS v(id, vals) JOIN users u ON u.id = v.id`, set)
	return stored, skipped, err
}

// resolvePeople maps every pushed subject that names a user to itself and
// every pushed verified email to its user, a batch of lookups at a time.
func resolvePeople(tx *gorm.DB, in []PersonEntitlements) (map[string]string, map[string]string, error) {
	var subjects, emails []string
	for _, person := range in {
		subjects = append(subjects, person.Subject)
		emails = append(emails, normalEmail(person.Email))
	}
	bySubject, err := lookup(tx, "SELECT id AS key, id FROM users WHERE id IN ?", subjects)
	if err != nil {
		return nil, nil, err
	}
	byEmail, err := lookup(tx, "SELECT lower(email) AS key, id FROM users WHERE lower(email) IN ? AND email_verified", emails)
	return bySubject, byEmail, err
}

// lookup runs query over keys in batches and answers key -> id.
func lookup(tx *gorm.DB, query string, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(keys); start += writeBatch {
		var rows []struct{ Key, ID string }
		if err := tx.Raw(query, keys[start:min(start+writeBatch, len(keys))]).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.Key] = row.ID
		}
	}
	delete(out, "")
	return out, nil
}

// insertScopes runs statement, whose %s takes a VALUES list of (id, json),
// over set in batches, and answers how many rows it stored.
func insertScopes(tx *gorm.DB, statement string, set scopeSet) (int, error) {
	stored := 0
	for start := 0; start < len(set.keys); start += writeBatch {
		batch := set.keys[start:min(start+writeBatch, len(set.keys))]
		args := make([]any, 0, 2*len(batch))
		for _, key := range batch {
			args = append(args, key, set.values[key])
		}
		placeholders := strings.TrimSuffix(strings.Repeat("(?, ?),", len(batch)), ",")
		result := tx.Exec(fmt.Sprintf(statement, placeholders), args...)
		if result.Error != nil {
			return 0, result.Error
		}
		stored += int(result.RowsAffected)
	}
	return stored, nil
}

// normalEmail is the form emails are compared in.
func normalEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
