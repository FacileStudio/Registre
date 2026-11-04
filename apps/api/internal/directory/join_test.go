package directory

import (
	"context"
	"errors"
)

// joining is one person entering a space: who invites, which space, the
// address, the role and the account that accepts.
type joining struct {
	actor, space, email, role, joiner string
}

// join invites and accepts, which is how a person enters a space now that
// membership needs their consent.
func join(ctx context.Context, svc *Service, in joining) error {
	actorID, spaceID, email, role, joinerID := in.actor, in.space, in.email, in.role, in.joiner
	if err := svc.Invite(ctx, actorID, spaceID, email, role); err != nil {
		return err
	}
	mine, err := svc.Invited(ctx, joinerID)
	if err != nil {
		return err
	}
	for _, invitation := range mine {
		if invitation.SpaceID == spaceID {
			_, err := svc.Accept(ctx, joinerID, invitation.ID)
			return err
		}
	}
	return errors.New("join: no invitation reached the joiner")
}
