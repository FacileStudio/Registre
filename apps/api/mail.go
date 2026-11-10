package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"

	mailer "github.com/FacileStudio/facteur"
)

// mailHandoff bounds one background delivery. It is generous because it is
// not on any request path: nothing waits for it, and an SMTP server that
// takes twenty seconds is slow rather than broken.
const mailHandoff = 30 * time.Second

// outboundMail returns the mailer the recovery service sends through, and
// whether SMTP is configured at all.
//
// Delivery happens on a goroutine, which is the point rather than an
// optimisation. recovery.Request equalises its own work between a known and
// an unknown address, but only the known address has a mail to send, so a
// synchronous SMTP round trip would put back exactly the timing oracle the
// equalisation removes: "the slow answer means that address exists". Handing
// off before the transport starts makes both paths return at the same moment
// whatever the mail server is doing.
//
// The cost is that a delivery failure reaches a log rather than the caller,
// which is the right trade here. The caller is a human who has been told a
// link is on its way whether or not their address was known, so there is no
// answer a failure could usefully change.
func outboundMail(cfg mailer.Config, log *slog.Logger) (recovery.Mailer, bool) {
	if cfg.Host == "" || cfg.FromEmail == "" {
		return nil, false
	}
	sender := mailer.New(cfg)
	return recovery.MailerFunc(func(_ context.Context, to, subject, html string) error {
		go deliver(sender, log, to, subject, html)
		return nil
	}), true
}

// deliver sends one message on its own goroutine, with a context of its own.
//
// The request's context is deliberately not carried in: it is cancelled the
// moment the response is written, which is before the send has begun, so
// every mail would be cancelled at birth.
func deliver(sender *mailer.Mailer, log *slog.Logger, to, subject, html string) {
	ctx, cancel := context.WithTimeout(context.Background(), mailHandoff)
	defer cancel()
	err := sender.Send(ctx, mailer.Message{To: []string{to}, Subject: subject, HTML: html, Text: i18n.PlainText(html)})
	if err != nil {
		log.Error("registre: failed to deliver a recovery mail", slog.Any("error", err))
	}
}

// recoveryService builds password recovery, or returns nil when this instance
// has no mailer.
//
// nil is a supported state rather than a boot failure. Recovery is the only
// feature in registre that depends on something outside it, and an instance
// whose operators mint every account by hand has no use for it. The routes
// still mount and still answer identically, so the absence is not something a
// caller can detect.
func recoveryService(cfg env.Config, p people) *recovery.Service {
	smtp := mailer.FromEnv(os.Getenv)
	outbound, configured := outboundMail(smtp, p.log)
	if !configured {
		p.log.Warn("registre: SMTP is not configured, password recovery will accept " +
			"requests and send nothing; set SMTP_HOST and SMTP_FROM to turn it on")
		return nil
	}
	p.log.Info("registre: password recovery is on", slog.String("smtp_host", smtp.Host))
	return recovery.New(p.db, p.verifier, outbound, cfg.Issuer).WithBreachCheck(p.self.pwned)
}
