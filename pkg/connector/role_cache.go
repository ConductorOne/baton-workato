package connector

import (
	"context"
	"strconv"
	"sync"

	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

type roleCache struct {
	client       *client.WorkatoClient
	folderToRole map[int][]*client.Role
	roles        map[string]*client.Role

	initialized bool
	mu          sync.Mutex
}

func newRoleCache(workatoClient *client.WorkatoClient) *roleCache {
	return &roleCache{
		client:       workatoClient,
		folderToRole: make(map[int][]*client.Role),
		roles:        make(map[string]*client.Role),
	}
}

func (p *roleCache) init(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	if err := p.buildCache(ctx); err != nil {
		return err
	}

	p.initialized = true
	return nil
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

	l.Info("Cache built for Roles", zap.Int("count", len(p.roles)))

	return nil
}

func (p *roleCache) getRoleByFolder(folderId int) []*client.Role {
	if !p.initialized {
		return nil
	}

	value, ok := p.folderToRole[folderId]
	if !ok {
		return nil
	}

	return value
}

func (p *roleCache) getRoleById(id string) *client.Role {
	if !p.initialized {
		return nil
	}

	value, ok := p.roles[id]
	if !ok {
		return nil
	}

	return value
}
