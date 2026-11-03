package spaces

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// list answers the caller's spaces.
func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.Mine(r.Context(), holderID(r))
	s.answer(w, err, out)
}

// create makes a space the caller owns.
func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &body) {
		return
	}
	out, err := s.spaces.Create(r.Context(), holderID(r), body.Name)
	s.answer(w, err, out)
}

// get answers one space with its members.
func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	out, err := s.spaces.Get(r.Context(), holderID(r), chi.URLParam(r, "spaceID"))
	s.answer(w, err, out)
}

// rename changes a space's name.
func (s *Service) rename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &body) {
		return
	}
	spaceID := chi.URLParam(r, "spaceID")
	err := s.spaces.Rename(r.Context(), holderID(r), spaceID, body.Name)
	s.answer(w, err, map[string]string{"id": spaceID, "name": body.Name})
}

// addMember invites an existing Registre user by email.
func (s *Service) addMember(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if !decode(w, r, &body) {
		return
	}
	member, err := s.spaces.AddMember(r.Context(), holderID(r), chi.URLParam(r, "spaceID"), body.Email, body.Role)
	s.answer(w, err, map[string]any{"member": member})
}

// setRole changes one member's rank.
func (s *Service) setRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &body) {
		return
	}
	spaceID, userID := chi.URLParam(r, "spaceID"), chi.URLParam(r, "userID")
	err := s.spaces.SetRole(r.Context(), holderID(r), spaceID, userID, body.Role)
	s.answer(w, err, map[string]any{"member": map[string]string{"user_id": userID, "role": body.Role}})
}

// removeMember takes one member out.
func (s *Service) removeMember(w http.ResponseWriter, r *http.Request) {
	err := s.spaces.RemoveMember(r.Context(), holderID(r), chi.URLParam(r, "spaceID"), chi.URLParam(r, "userID"))
	s.answer(w, err, map[string]bool{"ok": true})
}

// leave takes the caller out.
func (s *Service) leave(w http.ResponseWriter, r *http.Request) {
	err := s.spaces.Leave(r.Context(), holderID(r), chi.URLParam(r, "spaceID"))
	s.answer(w, err, map[string]bool{"ok": true})
}
