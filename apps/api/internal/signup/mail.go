package signup

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// Throttled is ErrThrottled with the wait until the oldest of the hour's
// links stops counting, so the page can say when to try again.
type Throttled struct{ Wait time.Duration }

// Error describes the refusal.
func (t *Throttled) Error() string { return ErrThrottled.Error() }

// Is makes errors.Is(err, ErrThrottled) hold.
func (t *Throttled) Is(target error) bool { return target == ErrThrottled }

// ErrMail is a mail the transport refused or could not reach. Both branches
// send one, so it says nothing about whether the address has an account.
// The link it would have carried is withdrawn, so a retry is not throttled.
var ErrMail = errors.New("signup: the mail could not be sent")

// MailTimeout bounds the SMTP round trip Start waits for. It sits under the
// HTTP server's write timeout so the person always gets an answer.
const MailTimeout = 10 * time.Second

// send delivers the mail within MailTimeout. On failure it withdraws the
// link row and answers ErrMail around the transport's reason with every
// address taken out, so the caller can log it.
func (s *Service) send(ctx context.Context, id, to, subject, body string) error {
	sendCtx, cancel := context.WithTimeout(ctx, MailTimeout)
	defer cancel()
	err := s.Mailer.Send(sendCtx, to, subject, body)
	if err == nil {
		return nil
	}
	if drop := s.DB.WithContext(context.WithoutCancel(ctx)).Delete(&schemas.SignupChallenge{}, "id = ?", id).Error; drop != nil {
		return fmt.Errorf("%w: %s; withdrawing the link: %s", ErrMail, Redact(err.Error()), Redact(drop.Error()))
	}
	return fmt.Errorf("%w: %s", ErrMail, Redact(err.Error()))
}

// anyAddress matches anything shaped like an email address.
var anyAddress = regexp.MustCompile(`[^\s<>"'(),;:]+@[^\s<>"'(),;:]+`)

// Redact replaces every email address in text, so an SMTP server's reply
// that repeats the recipient never reaches the logs.
func Redact(text string) string {
	return anyAddress.ReplaceAllString(text, "<address>")
}
