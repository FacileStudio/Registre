package recovery

import (
	"context"
	"errors"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
)

type everything struct{}

func (everything) Breached(context.Context, string) bool { return true }

// TestAResetRefusesABreachedPassword screens the emailed reset path too.
func TestAResetRefusesABreachedPassword(t *testing.T) {
	f := service(t)
	f.svc.WithBreachCheck(everything{})
	createUser(t, f.users, "human@example.com", "the current password")
	token := requestFor(t, f.svc, f.mailer, "human@example.com")
	if err := f.svc.Redeem(context.Background(), token, "a breached but long password"); !errors.Is(err, credentials.ErrPasswordBreached) {
		t.Fatalf("Redeem with a breached password = %v, want ErrPasswordBreached", err)
	}
}
