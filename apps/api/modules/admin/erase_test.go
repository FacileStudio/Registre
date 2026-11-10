package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestAdminDeleteQueuesTheAppsDeletionNotices proves an operator's delete
// goes through the same erasure as a person's own: every suite app gets one
// notice, pushed to its deletion_uri or listed for polling, so no app keeps
// an orphaned account. A third-party client gets none.
func TestAdminDeleteQueuesTheAppsDeletionNotices(t *testing.T) {
	f := newFixture(t)
	apps := map[string]string{"courrier": "https://courrier.test/api/registre/deletions", "agenda": ""}
	for clientID, uri := range apps {
		row := schemas.Client{ClientID: clientID, Subject: "sub-" + clientID, Name: clientID,
			Kind: schemas.KindApplication, DeletionURI: uri}
		if _, err := f.register.Upsert(context.Background(), row); err != nil {
			t.Fatalf("seeding %s: %v", clientID, err)
		}
	}
	stranger := schemas.Client{ClientID: "https://client.test/cimd", Subject: "sub-cimd", Name: "cimd",
		Kind: schemas.KindApplication, Public: true, Registration: schemas.RegistrationDocument}
	if err := f.db.Create(&stranger).Error; err != nil {
		t.Fatalf("seeding the third party: %v", err)
	}

	response, body := f.call(t, http.MethodPost, "/api/admin/users", map[string]any{
		"email": "leaving@example.com", "password": "secure password",
	}, true)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("creating answered %d: %v", response.StatusCode, body)
	}
	userID, _ := body["id"].(string)
	if response, body = f.call(t, http.MethodDelete, "/api/admin/users/"+userID, nil, true); response.StatusCode != http.StatusOK {
		t.Fatalf("deleting answered %d: %v", response.StatusCode, body)
	}

	var queued []string
	if err := f.db.Table("deletion_notices").Where("subject = ?", userID).Pluck("client_id", &queued).Error; err != nil {
		t.Fatalf("reading the notices: %v", err)
	}
	perApp := map[string]int{}
	for _, clientID := range queued {
		perApp[clientID]++
	}
	for clientID := range apps {
		if perApp[clientID] != 1 {
			t.Errorf("%s got %d notices, want exactly one", clientID, perApp[clientID])
		}
	}
	if perApp[stranger.ClientID] != 0 {
		t.Error("a third-party client was queued a deletion notice")
	}
	var left int64
	f.db.Model(&schemas.User{}).Where("id = ?", userID).Count(&left)
	if left != 0 {
		t.Error("the account survived the delete")
	}
}
