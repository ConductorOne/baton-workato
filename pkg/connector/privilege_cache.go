package connector

import (
	"context"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type CompoundUser struct {
	User       *client.Collaborator
	UserDetail *client.CollaboratorDetails
}

type privilegeCache struct {
	client          *client.WorkatoClient
	privilegeToUser map[string][]*CompoundUser
}

func newPrivilegeCache(workatoClient *client.WorkatoClient) *privilegeCache {
	return &privilegeCache{
		client:          workatoClient,
		privilegeToUser: make(map[string][]*CompoundUser),
	}
}

func (p *privilegeCache) buildCache(ctx context.Context) error {
	l := ctxzap.Extract(ctx)

	l.Info("Building cache for privileges")

	p.privilegeToUser = make(map[string][]*CompoundUser)

	collaborators, err := p.client.GetCollaborators(ctx)
	if err != nil {
		return err
	}

	for _, collaborator := range collaborators {
		collaboratorDetails, err := p.client.GetCollaboratorById(ctx, collaborator.Id)
		if err != nil {
			return err
		}

		for keyGroup, values := range collaboratorDetails.Privileges {
			for _, value := range values {
				privilegeKey := workato.PrivilegeId(keyGroup, value)

				p.privilegeToUser[privilegeKey] = append(p.privilegeToUser[privilegeKey], &CompoundUser{
					User:       &collaborator,
					UserDetail: collaboratorDetails,
				})
			}
		}
	}

	l.Info("Cache built for privileges")

	return nil
}

func (p *privilegeCache) getUsers(privilegeKey string) []*CompoundUser {
	value, ok := p.privilegeToUser[privilegeKey]
	if !ok {
		return make([]*CompoundUser, 0)
	}

	return value
}
