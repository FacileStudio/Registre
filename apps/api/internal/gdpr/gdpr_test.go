package gdpr

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

type spy struct {
	mu      sync.Mutex
	logouts []string
	deleted []string
	touched int
	mails   []string
	failing bool
}

func (s *spy) Fire(_ context.Context, userID string, clients []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logouts = append(s.logouts, userID)
	return nil
}

func (s *spy) DeliverDeleted(_ context.Context, userID, clientID, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failing {
		return errors.New("app down")
	}
	s.deleted = append(s.deleted, userID+"@"+clientID)
	return nil
}

func (s *spy) Touch(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.touched++
	return nil
}

func (s *spy) Send(_ context.Context, to, subject, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mails = append(s.mails, to+": "+subject)
	return nil
}

func (s *spy) EndSessions(ctx context.Context, userID string, at time.Time) ([]string, error) {
	return []string{"nuage"}, nil
}

func open(t *testing.T) (*Service, *gorm.DB, *spy) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	effects := &spy{}
	svc := New(Deps{
		DB: db, Sessions: effects, Audit: audit.New(db), Mailer: effects, Notifier: effects, Directory: effects,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, "https://sso.example.com")
	return svc, db, effects
}

func seed(t *testing.T, db *gorm.DB) schemas.User {
	t.Helper()
	user := schemas.User{ID: "gone", Email: "gone@example.com", Locale: "fr"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := db.Create(&schemas.Session{ID: "s1", UserID: "gone", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return user
}

// TestDeletionWaitsCanBeCancelledAndThenRuns walks the whole grace period.
func TestDeletionWaitsCanBeCancelledAndThenRuns(t *testing.T) {
	svc, db, effects := open(t)
	ctx := context.Background()
	user := seed(t, db)

	at, err := svc.Request(ctx, user)
	if err != nil || at.Before(time.Now().Add(Grace-time.Minute)) {
		t.Fatalf("Request = %s, %v; want about 30 days out", at, err)
	}
	if again, _ := svc.Request(ctx, user); !again.Equal(at) {
		t.Fatalf("a second request moved the date from %s to %s", at, again)
	}
	if svc.RunDue(ctx) != 0 {
		t.Fatal("an account inside its grace period was erased")
	}
	if err := svc.Cancel(ctx, user.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if pending, _ := svc.Scheduled(ctx, user.ID); pending != nil {
		t.Fatalf("still scheduled after cancel: %s", pending)
	}

	if _, err := svc.Request(ctx, user); err != nil {
		t.Fatalf("Request: %v", err)
	}
	db.Model(&schemas.DeletionRequest{}).Where("user_id = ?", user.ID).Update("execute_after", time.Now().Add(-time.Second))
	if erased := svc.RunDue(ctx); erased != 1 {
		t.Fatalf("RunDue erased %d, want 1", erased)
	}
	var users, sessions int64
	db.Model(&schemas.User{}).Where("id = ?", user.ID).Count(&users)
	db.Model(&schemas.Session{}).Where("user_id = ?", user.ID).Count(&sessions)
	if users != 0 || sessions != 0 {
		t.Fatalf("after erasure: %d users, %d sessions remain", users, sessions)
	}
	if len(effects.logouts) != 1 || effects.touched != 1 {
		t.Fatalf("side effects: %+v", effects)
	}
	var actions []string
	db.Model(&schemas.AdminEvent{}).Where("target_id = ?", user.ID).Order("id").Pluck("action", &actions)
	if len(actions) < 3 || actions[len(actions)-1] != "account.deleted" {
		t.Fatalf("audit trail %v", actions)
	}
}

// TestTheExportCarriesNoSecrets.
func TestTheExportCarriesNoSecrets(t *testing.T) {
	svc, db, _ := open(t)
	user := seed(t, db)
	user.PasswordHash = "$argon2id$secret"
	out, err := svc.Export(context.Background(), user)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if out.Account.Email != user.Email || !out.Account.HasPassword || len(out.Sessions) != 1 {
		t.Fatalf("Export = %+v", out)
	}
}
