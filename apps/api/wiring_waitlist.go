package main

import (
	"log/slog"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/internal/waitlist"
	waitlistroutes "github.com/FacileStudio/Registre/apps/api/modules/waitlist"

	troncenv "github.com/FacileStudio/tronc/env"
	"gorm.io/gorm"
)

// buildWaitlist builds the product waitlists over the ALTCHA key sign-up
// uses. The waitlist serves whenever that key is long enough, whether or not
// sign-up itself is open; without it every route answers 404.
func buildWaitlist(db *gorm.DB, settings env.Signup, issuer string, log *slog.Logger) *waitlistroutes.Service {
	origins := waitlistOrigins()
	if len(settings.AltchaKey) < env.MinAltchaKeyLength {
		log.Info("registre: the waitlist is off (ALTCHA_HMAC_KEY is missing or shorter than 32 bytes)")
		return waitlistroutes.New(nil, nil, origins, log)
	}
	list := waitlist.New(db, []byte(settings.AltchaKey))
	proofs := signup.New(signup.Deps{DB: db}, settings.AltchaKey, issuer)
	return waitlistroutes.New(list, proofs, origins, log)
}

// waitlistOrigins are the browser origins allowed to join, from
// WAITLIST_ORIGINS when set, else the Scribe and Pastille landings.
func waitlistOrigins() []string {
	raw := troncenv.String("WAITLIST_ORIGINS", "")
	if strings.TrimSpace(raw) == "" {
		return waitlistroutes.DefaultOrigins()
	}
	var origins []string
	for _, origin := range strings.Split(raw, ",") {
		if origin = strings.TrimSuffix(strings.TrimSpace(origin), "/"); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
