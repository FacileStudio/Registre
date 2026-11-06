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
	mu     sync.Mutex
	mails  map[string]string
	bodies map[string]string
}

func (o *outbox) Send(_ context.Context, to, subject, body string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.mails[to] = subject
	o.bodies[to] = body
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
	mails := &outbox{mails: map[string]string{}, bodies: map[string]string{}}
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
		if body := mails.bodies["stranger@example.com"]; open && (strings.Contains(body, "owner@facile.studio") || strings.Contains(body, "&#34;Facile&#34;")) {
			t.Fatalf("the join mail carries the sender's words: %s", body)
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

// TestAnInvitationDiesWithItsSpace is the reviewer's PoC: accepting an
// invitation into a deleted space must not create a membership.
func TestAnInvitationDiesWithItsSpace(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	if err := w.svc.Invite(ctx, w.sub["owner@facile.studio"], w.spaceID, "bob@facile.studio", "admin"); err != nil {
		t.Fatal(err)
	}
	var inv schemas.SpaceInvitation
	w.svc.db.Where("space_id = ?", w.spaceID).Take(&inv)
	if err := w.svc.Delete(ctx, w.sub["owner@facile.studio"], w.spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.svc.Accept(ctx, w.sub["bob@facile.studio"], inv.ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("accepting into a deleted space = %v, want ErrInvitationNotFound", err)
	}
	var members int64
	w.svc.db.Model(&schemas.SpaceMember{}).Where("space_id = ?", w.spaceID).Count(&members)
	if members > 0 {
		t.Fatalf("bob joined a deleted space: %d member rows", members)
	}
}

// TestInvitationMailIsCappedPerRecipient writes every invitation but mails
// one address at most MaxMailsPerRecipient times a day, and never puts the
// sender's words in a mail to a stranger.
func TestInvitationMailIsCappedPerRecipient(t *testing.T) {
	w := invitationWorld(t, true)
	ctx := context.Background()
	owner := w.sub["owner@facile.studio"]
	sent := 0
	for i := 0; i <= MaxMailsPerRecipient; i++ {
		space, err := w.svc.Create(ctx, owner, "Spam "+string(rune('A'+i)))
		if err != nil {
			t.Fatal(err)
		}
		delete(w.mails.mails, "target@example.com")
		if err := w.svc.Invite(ctx, owner, space.ID, "target@example.com", "member"); err != nil {
			t.Fatalf("invite %d: %v", i, err)
		}
		if w.mails.to("target@example.com") != "" {
			sent++
		}
	}
	if sent != MaxMailsPerRecipient {
		t.Fatalf("%d mails reached the target, want %d", sent, MaxMailsPerRecipient)
	}
}

// TestRefreshingAnInvitationCannotBypassTheMailCap is the reviewer's PoC:
// re-inviting one address rewrites one row, and must not mail it each time.
func TestRefreshingAnInvitationCannotBypassTheMailCap(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	sent := 0
	for i := 0; i < 10; i++ {
		w.mails.mails = map[string]string{}
		if err := w.svc.Invite(ctx, w.sub["owner@facile.studio"], w.spaceID, "bob@facile.studio", "member"); err != nil {
			t.Fatal(err)
		}
		if w.mails.to("bob@facile.studio") != "" {
			sent++
		}
	}
	if sent != MaxMailsPerRecipient {
		t.Fatalf("10 refreshes of one invitation sent %d mails, want %d", sent, MaxMailsPerRecipient)
	}
}

// TestAnInvitationIsOnlyAsGoodAsItsSender withdraws a removed or demoted
// inviter's pending invitations, and refuses one whose sender lost the rank.
func TestAnInvitationIsOnlyAsGoodAsItsSender(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	owner, eve := w.sub["owner@facile.studio"], w.sub["eve@facile.studio"]
	if err := join(ctx, w.svc, joining{owner, w.spaceID, "eve@facile.studio", "admin", eve}); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.Invite(ctx, eve, w.spaceID, "bob@facile.studio", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.SetRole(ctx, owner, w.spaceID, eve, "member"); err != nil {
		t.Fatal(err)
	}
	if mine, _ := w.svc.Invited(ctx, w.sub["bob@facile.studio"]); len(mine) != 0 {
		t.Fatalf("a demoted inviter's invitation survived: %+v", mine)
	}

	if err := w.svc.SetRole(ctx, owner, w.spaceID, eve, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.Invite(ctx, eve, w.spaceID, "bob@facile.studio", "admin"); err != nil {
		t.Fatal(err)
	}
	w.svc.db.Exec("UPDATE space_members SET role = 'member' WHERE user_id = ?", eve)
	mine, _ := w.svc.Invited(ctx, w.sub["bob@facile.studio"])
	if len(mine) != 1 {
		t.Fatalf("bob has %d invitations, want 1", len(mine))
	}
	if _, err := w.svc.Accept(ctx, w.sub["bob@facile.studio"], mine[0].ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("accepting from a sender who lost the rank = %v, want ErrInvitationNotFound", err)
	}

	w.svc.db.Exec("UPDATE space_members SET role = 'admin' WHERE user_id = ?", eve)
	if err := w.svc.Invite(ctx, eve, w.spaceID, "bob@facile.studio", "member"); err != nil {
		t.Fatal(err)
	}
	if err := w.svc.RemoveMember(ctx, owner, w.spaceID, eve); err != nil {
		t.Fatal(err)
	}
	if mine, _ := w.svc.Invited(ctx, w.sub["bob@facile.studio"]); len(mine) != 0 {
		t.Fatalf("a removed inviter's invitation survived: %+v", mine)
	}
}

// TestInvitationMailCarriesNoSenderTextAndIsCappedPerInviter.
func TestInvitationMailCarriesNoSenderTextAndIsCappedPerInviter(t *testing.T) {
	w := invitationWorld(t, true)
	ctx := context.Background()
	owner := w.sub["owner@facile.studio"]
	w.svc.db.Exec("UPDATE users SET name = 'Eve <script>' WHERE id = ?", owner)
	w.svc.db.Exec("UPDATE spaces SET name = 'Click http://evil.example' WHERE id = ?", w.spaceID)
	if err := w.svc.Invite(ctx, owner, w.spaceID, "bob@facile.studio", "member"); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"Eve", "evil.example", "owner@facile.studio"} {
		if strings.Contains(w.mails.bodies["bob@facile.studio"], leak) {
			t.Fatalf("the invitation mail carries %q: %s", leak, w.mails.bodies["bob@facile.studio"])
		}
	}
	sent := 1
	for i := 0; i < MaxMailsPerInviter+5; i++ {
		address := "stranger" + string(rune('a'+i)) + "@example.com"
		if err := w.svc.Invite(ctx, owner, w.spaceID, address, "member"); err != nil {
			t.Fatalf("invite %d: %v", i, err)
		}
		if w.mails.to(address) != "" {
			sent++
		}
	}
	if sent != MaxMailsPerInviter {
		t.Fatalf("one inviter sent %d mails in a day, want %d", sent, MaxMailsPerInviter)
	}
}

// TestADisabledInvitersInvitationIsRefused.
func TestADisabledInvitersInvitationIsRefused(t *testing.T) {
	w := invitationWorld(t, false)
	ctx := context.Background()
	if err := w.svc.Invite(ctx, w.sub["owner@facile.studio"], w.spaceID, "bob@facile.studio", "member"); err != nil {
		t.Fatal(err)
	}
	w.svc.db.Exec("UPDATE users SET disabled_at = now() WHERE id = ?", w.sub["owner@facile.studio"])
	mine, _ := w.svc.Invited(ctx, w.sub["bob@facile.studio"])
	if len(mine) != 1 {
		t.Fatalf("bob has %d invitations, want 1", len(mine))
	}
	if _, err := w.svc.Accept(ctx, w.sub["bob@facile.studio"], mine[0].ID); !errors.Is(err, ErrInvitationNotFound) {
		t.Fatalf("accepting a disabled inviter's invitation = %v, want ErrInvitationNotFound", err)
	}
}
