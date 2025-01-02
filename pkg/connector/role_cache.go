package connector

import (
	"context"

	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type roleCache struct {
	client       *client.WorkatoClient
	folderToRole map[int][]*client.Role
}

func newRoleCache(workatoClient *client.WorkatoClient) *roleCache {
	return &roleCache{
		client:       workatoClient,
		folderToRole: make(map[int][]*client.Role),
	}
}

func (p *roleCache) buildCache(ctx context.Context) error {
	l := ctxzap.Extract(ctx)

	l.Info("Building cache for Roles")

	p.folderToRole = make(map[int][]*client.Role)

	token := ""

	for {
		roles, nextToken, err := p.client.GetRoles(ctx, token)
		if err != nil {
			return err
		}

		if nextToken == "" {
			break
		}

		for _, role := range roles {
			for _, folderID := range role.FolderIDs {
				copyRole := role
				p.folderToRole[folderID] = append(p.folderToRole[folderID], &copyRole)
			}
		}

		token = nextToken
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
