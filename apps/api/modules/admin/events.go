package admin

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// eventView is one admin event, with the operator who made it.
type eventView struct {
	ID         int64          `json:"id"`
	At         string         `json:"at"`
	Actor      *userView      `json:"actor,omitempty"`
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Detail     map[string]any `json:"detail"`
}

// listEvents answers the activity log, newest first, fifty at a time.
func (s *Service) listEvents(w http.ResponseWriter, r *http.Request) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	rows, err := s.audit.List(r.Context(), before, 50)
	if err != nil {
		serverError(w, err)
		return
	}
	actors := map[string]*userView{}
	views := make([]eventView, 0, len(rows))
	for _, row := range rows {
		views = append(views, eventView{
			ID: row.ID, At: row.At.UTC().Format(timeFormat), Actor: s.actor(r, actors, row.ActorID),
			ActorID: row.ActorID, Action: row.Action, TargetType: row.TargetType, TargetID: row.TargetID,
			Detail: row.Detail,
		})
	}
	encode(w, views, http.StatusOK)
}

func (s *Service) actor(r *http.Request, seen map[string]*userView, id string) *userView {
	if view, ok := seen[id]; ok {
		return view
	}
	seen[id] = nil
	user, err := s.users.ByID(r.Context(), id)
	if err != nil {
		return nil
	}
	views, err := s.userViews(r, []schemas.User{user})
	if err != nil {
		return nil
	}
	seen[id] = &views[0]
	return seen[id]
}

func userTarget(id string) audit.Target   { return audit.Target{Type: "user", ID: id} }
func clientTarget(id string) audit.Target { return audit.Target{Type: "client", ID: id} }

// record writes an admin event for the operator of r. A failure is logged and
// swallowed: the change already happened, and refusing the response would
// make the operator retry something that succeeded.
func (s *Service) record(r *http.Request, action string, target audit.Target, detail map[string]any) {
	if s.audit == nil {
		return
	}
	operator := r.Context().Value(operatorKey).(schemas.User)
	if err := s.audit.Record(r.Context(), operator.ID, action, target, detail); err != nil {
		s.logger.Warn("registre: could not record an admin event", slog.String("action", action), slog.Any("error", err))
	}
}
