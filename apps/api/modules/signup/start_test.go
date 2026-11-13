package signup

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
)

// TestAMailThatCannotBeSentIsNeverCalledSent: a refused mail answers 503
// mail_failed for a known and an unknown address alike.
func TestAMailThatCannotBeSentIsNeverCalledSent(t *testing.T) {
	refuse := recovery.MailerFunc(func(context.Context, string, string, string) error {
		return errors.New("421 try again later")
	})
	router := openWith(t, DefaultLimits(), refuse)
	for _, email := range []string{"known@example.com", "stranger@example.com"} {
		status, body := call(t, router, http.MethodPost, "/api/signup/start", `{"email":"`+email+`","altcha":"`+proof(t, router)+`"}`)
		if status != http.StatusServiceUnavailable || !strings.Contains(body, "mail_failed") {
			t.Fatalf("start for %s with a refusing server = %d %s, want 503 mail_failed", email, status, body)
		}
	}
}
