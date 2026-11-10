package signup

import (
	"net/url"
	"regexp"
)

// invitationID is the shape of a space invitation id.
var invitationID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// loginLink is the sign-in page, leading on to the invitation when one rode along.
func (s *Service) loginLink(invitation string) string {
	if !invitationID.MatchString(invitation) {
		return s.issuer + "/login"
	}
	return s.issuer + "/login?next=" + url.QueryEscape("/invitations/"+invitation)
}

// finishLink is the mailed sign-up link, carrying the invitation when one rode along.
func (s *Service) finishLink(token, invitation string) string {
	link := s.issuer + "/signup/finish?token=" + url.QueryEscape(token)
	if invitationID.MatchString(invitation) {
		link += "&invitation=" + invitation
	}
	return link
}
