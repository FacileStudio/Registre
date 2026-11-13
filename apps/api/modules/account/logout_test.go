package account

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// logouts records the apps each back-channel logout was fired to.
type logouts chan []string

// Fire records one logout.
func (l logouts) Fire(_ context.Context, _ string, clientIDs []string) error {
	l <- clientIDs
	return nil
}

func TestLoggingOutTellsEveryAppTheSessionReachedOnce(t *testing.T) {
	f := newFixture(t)
	err := f.db.Model(&schemas.Session{}).Where("user_id = ?", f.user).
		Update("client_ids", schemas.StringList{"opus"}).Error
	if err != nil {
		t.Fatalf("recording the app: %v", err)
	}

	for attempt := range 2 {
		response, _ := f.send(t, http.MethodPost, "/api/account/logout", nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("logout %d answered %d, want 200", attempt+1, response.StatusCode)
		}
	}
	select {
	case clients := <-f.fired:
		if len(clients) != 1 || clients[0] != "opus" {
			t.Fatalf("fired to %v, want [opus]", clients)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("logging out told no app")
	}
	select {
	case clients := <-f.fired:
		t.Fatalf("the dead cookie fired a second logout to %v", clients)
	case <-time.After(200 * time.Millisecond):
	}
}
