package directory

import (
	"encoding/json"
	"sort"

	wire "github.com/FacileStudio/porte/directory"
	"gorm.io/gorm"
)

// operatorCatalog is every feature Ardoise has pushed, as the full
// entitlement set of an unrestricted account: true for a bool, -1 (unlimited)
// for a limit. Ardoise pushes every declared feature to every scope it
// resolves, so the keys and kinds stored in space_entitlements and
// user_entitlements are the catalog. A key stored as a bool in one row and a
// number in another is dropped, and so is any other kind, so a drifting
// catalog can only withhold a feature, never grant a wrong one. Nothing
// stored means an empty catalog.
func operatorCatalog(tx *gorm.DB) (map[string]json.RawMessage, error) {
	var rows []struct{ Key, Kind string }
	err := tx.Raw(`SELECT DISTINCT j.key AS key, jsonb_typeof(j.value) AS kind FROM (
			SELECT "values" FROM space_entitlements UNION ALL SELECT "values" FROM user_entitlements
		) e, jsonb_each(e."values") j`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	kinds := map[string]string{}
	for _, row := range rows {
		if prior, seen := kinds[row.Key]; seen && prior != row.Kind {
			kinds[row.Key] = "conflict"
			continue
		}
		kinds[row.Key] = row.Kind
	}
	out := map[string]json.RawMessage{}
	for key, kind := range kinds {
		switch kind {
		case "boolean":
			out[key] = json.RawMessage("true")
		case "number":
			out[key] = json.RawMessage("-1")
		}
	}
	return out, nil
}

// operatorIDs lists the enabled operators: users.admin and not disabled,
// the same test the admin API applies to a caller.
func operatorIDs(tx *gorm.DB) ([]string, error) {
	var ids []string
	err := tx.Raw(`SELECT id FROM users WHERE admin AND disabled_at IS NULL`).Scan(&ids).Error
	return ids, err
}

// withOperators gives every enabled operator the whole catalog as their
// personal entitlements, whatever Ardoise pushed for them, and answers the
// people sorted by subject. It changes nothing for anybody else, and nothing
// at all while the catalog is empty, so a Registre that holds no pushed
// entitlements never grants a paid feature to an operator or anyone.
func withOperators(tx *gorm.DB, people []wire.Person) ([]wire.Person, error) {
	catalog, err := operatorCatalog(tx)
	if err != nil || len(catalog) == 0 {
		return people, err
	}
	ids, err := operatorIDs(tx)
	if err != nil || len(ids) == 0 {
		return people, err
	}
	operator := make(map[string]bool, len(ids))
	for _, id := range ids {
		operator[id] = true
	}
	out := make([]wire.Person, 0, len(people)+len(ids))
	for _, person := range people {
		if !operator[person.Subject] {
			out = append(out, person)
		}
	}
	for _, id := range ids {
		out = append(out, wire.Person{Subject: id, Entitlements: catalog})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Subject < out[j].Subject })
	return out, nil
}
