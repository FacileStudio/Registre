package directory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// TestTheInvitationMailNamesWhoWhatAndAsWhat is the GitHub-style mail: who
// invited, to which space, as which role, in the invitee's language, with a
// button to the invitation's own page, and a plain-text part that keeps the
// link.
func TestTheInvitationMailNamesWhoWhatAndAsWhat(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	owner := w.sub["owner@facile.studio"]
	w.svc.db.Exec("UPDATE users SET name = 'Noah' WHERE id = ?", owner)
	w.svc.db.Exec("UPDATE users SET locale = 'fr' WHERE id = ?", w.sub["bob@facile.studio"])
	if err := w.svc.Invite(ctx, owner, w.spaceID, "bob@facile.studio", "admin"); err != nil {
		t.Fatal(err)
	}
	var row schemas.SpaceInvitation
	w.svc.db.Where("space_id = ?", w.spaceID).Take(&row)
	subject, body := w.mails.to("bob@facile.studio"), w.mails.bodies["bob@facile.studio"]
	if subject != "Noah vous invite à rejoindre Facile sur Facile" {
		t.Fatalf("subject = %q", subject)
	}
	link := "https://sso.example.com/invitations/" + row.ID
	for _, want := range []string{"« Facile »", "en tant que administrateur", link, "Voir l&#39;invitation", i18n.Date(row.ExpiresAt, "fr")} {
		if !strings.Contains(body, want) {
			t.Errorf("the mail lacks %q: %s", want, body)
		}
	}
	text := i18n.PlainText(body)
	if !strings.Contains(text, "Voir l'invitation: "+link) || strings.Contains(text, "&#39;") || strings.Contains(text, "<") {
		t.Fatalf("the plain-text part = %q", text)
	}
}

// TestARefreshedInvitationKeepsItsLink re-invites one address and expects
// the second mail to link to the same row, not to the id the refused insert
// minted.
func TestARefreshedInvitationKeepsItsLink(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	owner := w.sub["owner@facile.studio"]
	for i := 0; i < 2; i++ {
		if err := w.svc.Invite(ctx, owner, w.spaceID, "bob@facile.studio", "member"); err != nil {
			t.Fatal(err)
		}
	}
	var row schemas.SpaceInvitation
	w.svc.db.Where("space_id = ?", w.spaceID).Take(&row)
	if !strings.Contains(w.mails.bodies["bob@facile.studio"], "/invitations/"+row.ID) {
		t.Fatalf("the refreshed mail does not link to row %s: %s", row.ID, w.mails.bodies["bob@facile.studio"])
	}
}

// TestReadingAnInvitationChangesNothingAndNamesItsState covers the page's
// states: the addressee reads it without joining, someone else is told it
// is not theirs, an expired one says so, and a withdrawn one is gone.
func TestReadingAnInvitationChangesNothingAndNamesItsState(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	owner, bob, eve := w.sub["owner@facile.studio"], w.sub["bob@facile.studio"], w.sub["eve@facile.studio"]
	if err := w.svc.Invite(ctx, owner, w.spaceID, "bob@facile.studio", "admin"); err != nil {
		t.Fatal(err)
	}
	var row schemas.SpaceInvitation
	w.svc.db.Where("space_id = ?", w.spaceID).Take(&row)

	for i := 0; i < 3; i++ {
		got, err := w.svc.Invitation(ctx, bob, row.ID)
		if err != nil || got.SpaceName != "Facile" || got.Role != "admin" || got.InvitedBy != "owner@facile.studio" {
			t.Fatalf("bob reading his invitation = %+v, %v", got, err)
		}
	}
	if _, _, err := w.svc.membership(w.svc.db, bob, w.spaceID); err == nil {
		t.Fatal("reading the invitation made bob a member")
	}
	if mine, _ := w.svc.Invited(ctx, bob); len(mine) != 1 {
		t.Fatalf("reading the invitation consumed it: %+v", mine)
	}
	if got, err := w.svc.Invitation(ctx, eve, row.ID); !errors.Is(err, ErrInvitationWrongAccount) || got.SpaceName != "" {
		t.Fatalf("eve reading bob's invitation = %+v, %v, want ErrInvitationWrongAccount and nothing else", got, err)
	}

	w.svc.db.Model(&schemas.SpaceInvitation{}).Where("id = ?", row.ID).Update("expires_at", time.Now().Add(-time.Hour))
	if _, err := w.svc.Invitation(ctx, bob, row.ID); !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("reading an expired invitation = %v, want ErrInvitationExpired", err)
	}
	if err := w.svc.PurgeInvitations(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Invitation(ctx, bob, row.ID); !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("an hour-old expired invitation was purged: %v", err)
	}

	if err := w.svc.Revoke(ctx, owner, w.spaceID, row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Invitation(ctx, bob, row.ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("reading a withdrawn invitation = %v, want ErrInvitationNotFound", err)
	}
}
