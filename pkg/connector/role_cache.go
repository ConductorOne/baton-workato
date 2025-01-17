package connector

import (
	"context"
	"strconv"

	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type roleCache struct {
	client       *client.WorkatoClient
	folderToRole map[int][]*client.Role
	roles        map[string]*client.Role
}

func newRoleCache(workatoClient *client.WorkatoClient) *roleCache {
	return &roleCache{
		client:       workatoClient,
		folderToRole: make(map[int][]*client.Role),
		roles:        make(map[string]*client.Role),
	}
}

func (p *roleCache) buildCache(ctx context.Context) error {
	l := ctxzap.Extract(ctx)

	l.Info("Building cache for Roles")

	p.folderToRole = make(map[int][]*client.Role)
	p.roles = make(map[string]*client.Role)

	token := ""

	for {
		roles, nextToken, err := p.client.GetRoles(ctx, token)
		if err != nil {
			return err
		}

		for _, role := range roles {
			copyRole := role
			for _, folderID := range role.FolderIDs {
				p.folderToRole[folderID] = append(p.folderToRole[folderID], &copyRole)
			}

			p.roles[strconv.Itoa(role.Id)] = &copyRole
		}

		token = nextToken

		if nextToken == "" {
			break
		}
	}

	l.Info("Cache built for Roles")

	return nil
}

func (p *roleCache) getRoleByFolder(folderId int) []*client.Role {
	value, ok := p.folderToRole[folderId]
	if !ok {
		return make([]*client.Role, 0)
	}

	return value
}

func (p *roleCache) getRoleById(id string) *client.Role {
	value, ok := p.roles[id]
	if !ok {
		return nil
	}

	return value
}
