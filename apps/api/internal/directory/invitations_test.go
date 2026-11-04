package directory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

type outbox struct {
	mu    sync.Mutex
	mails map[string]string
}

func (o *outbox) Send(_ context.Context, to, subject, _ string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.mails[to] = subject
	return nil
}

func (o *outbox) to(address string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.mails[address]
}

// invites is a space with three accounts and a recording mailer.
type invites struct {
	svc     *Service
	sub     map[string]string
	spaceID string
	mails   *outbox
}

func invitationWorld(t *testing.T, signupOpen bool) invites {
	t.Helper()
	svc, _, sub := fixture(t, "owner@facile.studio", "bob@facile.studio", "eve@facile.studio")
	mails := &outbox{mails: map[string]string{}}
	svc.WithInvitations(Invitations{Mailer: mails, Issuer: "https://sso.example.com", SignupOpen: signupOpen})
	space, err := svc.Create(context.Background(), sub["owner@facile.studio"], "Facile")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return invites{svc: svc, sub: sub, spaceID: space.ID, mails: mails}
}

// TestAnInvitationIsNotAMembershipUntilAccepted keeps pending people out of
// every snapshot and lets only the addressee accept.
func TestAnInvitationIsNotAMembershipUntilAccepted(t *testing.T) {
	w := invitationWorld(t, false)
	svc, sub, spaceID, mails := w.svc, w.sub, w.spaceID, w.mails
	ctx := context.Background()
	if err := svc.Invite(ctx, sub["owner@facile.studio"], spaceID, "Bob@Facile.studio", "admin"); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	if mails.to("bob@facile.studio") == "" {
		t.Fatal("bob was not told about the invitation")
	}
	snapshot, _ := svc.Snapshot(ctx)
	if roles := rolesIn(t, snapshot); roles["bob@facile.studio"] != "" {
		t.Fatalf("a pending invitee is in the snapshot: %v", roles)
	}
	mine, _ := svc.Invited(ctx, sub["bob@facile.studio"])
	if len(mine) != 1 || mine[0].Role != "admin" || mine[0].SpaceName != "Facile" {
		t.Fatalf("bob's invitations: %+v", mine)
	}
	if _, err := svc.Accept(ctx, sub["eve@facile.studio"], mine[0].ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("eve accepting bob's invitation = %v, want ErrInvitationNotFound", err)
	}
	if _, err := svc.Accept(ctx, sub["bob@facile.studio"], mine[0].ID); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	after, _ := svc.Snapshot(ctx)
	if roles := rolesIn(t, after); roles["bob@facile.studio"] != "admin" || after.Version <= snapshot.Version {
		t.Fatalf("after accepting: %v at version %d", roles, after.Version)
	}
	if _, err := svc.Accept(ctx, sub["bob@facile.studio"], mine[0].ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("accepting twice = %v, want ErrInvitationNotFound", err)
	}
}

// TestUnknownAddressesGetTheSameAnswer never says whether an address has an
// account, and mails a stranger only while sign-up is open.
func TestUnknownAddressesGetTheSameAnswer(t *testing.T) {
	for _, open := range []bool{false, true} {
		w := invitationWorld(t, open)
		svc, sub, spaceID, mails := w.svc, w.sub, w.spaceID, w.mails
		err := svc.Invite(context.Background(), sub["owner@facile.studio"], spaceID, "stranger@example.com", "member")
		if err != nil {
			t.Fatalf("inviting an unknown address = %v, want nil", err)
		}
		got := mails.to("stranger@example.com")
		if open && !strings.Contains(got, "Facile") {
			t.Fatalf("sign-up open: the stranger got %q, want the join-Facile mail", got)
		}
		if !open && got != "" {
			t.Fatalf("sign-up closed: the stranger got %q, want nothing", got)
		}
	}
}

// TestInvitationsAreBoundedRevocableAndExpire.
func TestInvitationsAreBoundedRevocableAndExpire(t *testing.T) {
	w := invitationWorld(t, false)
	svc, sub, spaceID := w.svc, w.sub, w.spaceID
	ctx := context.Background()
	owner := sub["owner@facile.studio"]
	if err := svc.Invite(ctx, sub["bob@facile.studio"], spaceID, "eve@facile.studio", "member"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member inviting = %v, want ErrNotFound", err)
	}
	if err := svc.Invite(ctx, owner, spaceID, "owner@facile.studio", "member"); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("inviting a member = %v, want ErrAlreadyMember", err)
	}
	for i := 0; i < MaxPendingPerSpace; i++ {
		if err := svc.Invite(ctx, owner, spaceID, "p"+string(rune('a'+i%26))+string(rune('a'+i/26))+"@example.com", "member"); err != nil {
			t.Fatalf("invite %d: %v", i, err)
		}
	}
	if err := svc.Invite(ctx, owner, spaceID, "onemore@example.com", "member"); !errors.Is(err, ErrTooManyInvitations) {
		t.Fatalf("past the cap = %v, want ErrTooManyInvitations", err)
	}
	detail, _ := svc.Get(ctx, owner, spaceID)
	if len(detail.Invitations) != MaxPendingPerSpace {
		t.Fatalf("the owner sees %d pending, want %d", len(detail.Invitations), MaxPendingPerSpace)
	}
	if err := svc.Revoke(ctx, owner, spaceID, detail.Invitations[0].ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := svc.Invite(ctx, owner, spaceID, "bob@facile.studio", "member"); err != nil {
		t.Fatalf("Invite after a revoke: %v", err)
	}
	svc.db.Model(&schemas.SpaceInvitation{}).Where("email = ?", "bob@facile.studio").
		Update("expires_at", time.Now().Add(-time.Second))
	if mine, _ := svc.Invited(ctx, sub["bob@facile.studio"]); len(mine) != 0 {
		t.Fatalf("an expired invitation is still offered: %+v", mine)
	}
}
