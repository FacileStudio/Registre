package spaces

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/billing"
	"github.com/FacileStudio/Registre/apps/api/internal/directory"

	wire "github.com/FacileStudio/porte/directory"
	"github.com/go-chi/chi/v5"
)

// lookupKey is the shape of a Stripe lookup key such as plan_pro_year.
// Anything else is refused before Ardoise is called.
func lookupKey() *regexp.Regexp { return regexp.MustCompile(`^[a-z0-9_]{1,64}$`) }

// planView is the plan section of one space. Enabled is false when this
// instance has no billing configured, and the page hides the section.
// Available is false when Ardoise could not be reached. Catalog is only
// filled for owners, the only ones who can buy.
type planView struct {
	Enabled   bool               `json:"enabled"`
	Available bool               `json:"available"`
	Owner     bool               `json:"owner"`
	Plan      *billing.SpacePlan `json:"plan,omitempty"`
	Catalog   []billing.Plan     `json:"catalog,omitempty"`
}

// plan answers the space's plan to any member, and the catalog to owners.
func (s *Service) plan(w http.ResponseWriter, r *http.Request) {
	detail, err := s.spaces.Get(r.Context(), holderID(r), chi.URLParam(r, "spaceID"))
	if err != nil {
		s.answer(w, err, nil)
		return
	}
	if s.billing == nil {
		encode(w, planView{}, http.StatusOK)
		return
	}
	encode(w, s.lookup(r.Context(), detail), http.StatusOK)
}

// lookup asks Ardoise for the plan and, for an owner, the catalog. Either
// failing marks the section unavailable rather than half-drawn.
func (s *Service) lookup(ctx context.Context, detail directory.Detail) planView {
	view := planView{Enabled: true, Owner: detail.Role == wire.RoleOwner}
	current, err := s.billing.Space(ctx, detail.ID)
	if err != nil {
		s.logger.Warn("registre: billing plan lookup failed", slog.Any("error", err))
		return view
	}
	if view.Owner {
		catalog, err := s.billing.Catalog(ctx)
		if err != nil {
			s.logger.Warn("registre: billing catalog lookup failed", slog.Any("error", err))
			return view
		}
		view.Catalog = catalog.Plans
	}
	view.Plan, view.Available = &current, true
	return view
}

// checkout starts a Stripe checkout for one published price. Owners only;
// everything but the lookup key comes from server-held state.
func (s *Service) checkout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LookupKey string `json:"lookup_key"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !lookupKey().MatchString(body.LookupKey) {
		encode(w, map[string]string{"error": "unknown price"}, http.StatusBadRequest)
		return
	}
	detail, ok := s.owner(w, r)
	if !ok {
		return
	}
	user, _ := accounts.Holder(r.Context())
	back := s.spaceURL(detail.ID)
	target, err := s.billing.Checkout(r.Context(), billing.CheckoutRequest{
		SpaceID: detail.ID, SpaceName: detail.Name, OwnerEmail: user.Email,
		LookupKey: body.LookupKey, SuccessURL: back + "?billing=success", CancelURL: back,
	})
	s.redirect(w, target, err)
}

// portal opens the Stripe billing portal for the space. Owners only.
func (s *Service) portal(w http.ResponseWriter, r *http.Request) {
	detail, ok := s.owner(w, r)
	if !ok {
		return
	}
	target, err := s.billing.Portal(r.Context(), detail.ID, s.spaceURL(detail.ID))
	s.redirect(w, target, err)
}

// owner proves the caller owns the space, answering 404 when billing is off
// or they are not in it and 403 when they are in it below owner.
func (s *Service) owner(w http.ResponseWriter, r *http.Request) (directory.Detail, bool) {
	if s.billing == nil {
		encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return directory.Detail{}, false
	}
	detail, err := s.spaces.Get(r.Context(), holderID(r), chi.URLParam(r, "spaceID"))
	if err == nil && detail.Role != wire.RoleOwner {
		err = directory.ErrForbidden
	}
	if err != nil {
		s.answer(w, err, nil)
		return directory.Detail{}, false
	}
	return detail, true
}

// redirect answers the Stripe URL for the page to follow, or 502.
func (s *Service) redirect(w http.ResponseWriter, target string, err error) {
	if err != nil {
		s.logger.Warn("registre: billing session failed", slog.Any("error", err))
		encode(w, map[string]string{"error": billing.ErrUnavailable.Error()}, http.StatusBadGateway)
		return
	}
	encode(w, map[string]string{"url": target}, http.StatusOK)
}

// spaceURL is the space's page on this instance, built from the issuer so
// no request value ever chooses where Stripe sends the human back.
func (s *Service) spaceURL(spaceID string) string {
	return strings.TrimSuffix(s.issuer, "/") + "/spaces/" + url.PathEscape(spaceID)
}
