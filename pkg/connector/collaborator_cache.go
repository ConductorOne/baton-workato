package connector

import (
	"context"
	"strconv"

	"github.com/conductorone/baton-workato/pkg/connector/ucache"

	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
)

type CompoundUser struct {
	User       *client.Collaborator
	UserDetail []*client.CollaboratorPrivilege
}

func (c *CompoundUser) Id() string {
	return strconv.Itoa(c.User.Id)
}

type collaboratorCache struct {
	client          *client.WorkatoClient
	privilegeToUser *ucache.HashSet[string, string, CompoundUser]
	folderToUser    *ucache.HashSet[int, string, CompoundUser]
	roleToUser      *ucache.HashSet[string, string, CompoundUser]
	env             workato.Environment
}

func newCollaboratorCache(workatoClient *client.WorkatoClient, env workato.Environment) *collaboratorCache {
	return &collaboratorCache{
		client:          workatoClient,
		privilegeToUser: ucache.NewUCache[string, string, CompoundUser](),
		folderToUser:    ucache.NewUCache[int, string, CompoundUser](),
		roleToUser:      ucache.NewUCache[string, string, CompoundUser](),
		env:             env,
	}
}

func (p *collaboratorCache) buildCache(ctx context.Context) error {
	l := ctxzap.Extract(ctx)

	l.Info("Building cache for collaborators")

	p.privilegeToUser = ucache.NewUCache[string, string, CompoundUser]()
	p.folderToUser = ucache.NewUCache[int, string, CompoundUser]()
	p.roleToUser = ucache.NewUCache[string, string, CompoundUser]()

	collaborators, err := p.client.GetCollaborators(ctx)
	if err != nil {
		return err
	}

	for _, collaborator := range collaborators {
		collaboratorRoles, err := p.client.GetCollaboratorPrivileges(ctx, collaborator.Id)
		if err != nil {
			return err
		}

		compoundUser := &CompoundUser{
			User:       &collaborator,
			UserDetail: collaboratorRoles,
		}

		for _, collaboratorRole := range collaboratorRoles {
			if collaboratorRole.EnvironmentType != p.env.String() {
				continue
			}

			// Build for privileges
			for keyGroup, values := range collaboratorRole.Privileges {
				for _, value := range values {
					privilegeKey := workato.PrivilegeId(keyGroup, value)

					p.privilegeToUser.Set(privilegeKey, compoundUser.Id(), compoundUser)
				}
			}

			// Build for folders
			for _, folderId := range collaboratorRole.FolderIDs {
				p.folderToUser.Set(folderId, compoundUser.Id(), compoundUser)
			}
		}

		// Build for roles
		for _, role := range collaborator.Roles {
			if role.EnvironmentType != p.env.String() {
				continue
			}

			p.roleToUser.Set(role.RoleName, compoundUser.Id(), compoundUser)
		}
	}

	l.Info("Cache built for collaborators")

	return nil
}

// GetAllFoldersRecur is a recursive function that gets all folders in a Workato instance.
func (p *collaboratorCache) GetAllFoldersRecur(ctx context.Context, parentId *int, pToken string) ([]client.Folder, error) {
	l := ctxzap.Extract(ctx)

	l.Info("Building cache for folders")

	folders, nextToken, err := p.client.GetFolders(ctx, parentId, pToken)
	if err != nil {
		return nil, err
	}

	response := make([]client.Folder, 0)

	if len(folders) == 0 {
		return folders, nil
	} else {
		recurResult, err := p.GetAllFoldersRecur(ctx, parentId, nextToken)
		if err != nil {
			return nil, err
		}

		response = append(response, recurResult...)
	}

	for _, folder := range folders {
		recurResult, err := p.GetAllFoldersRecur(ctx, &folder.Id, "0")
		if err != nil {
			return nil, err
		}

		response = append(response, recurResult...)
	}

	return response, nil
}

func (p *collaboratorCache) getUsersByPrivilege(privilegeKey string) []*CompoundUser {
	return p.privilegeToUser.GetAll(privilegeKey)
}

func (p *collaboratorCache) getUsersByFolder(folderId int) []*CompoundUser {
	return p.folderToUser.GetAll(folderId)
}

func (p *collaboratorCache) getUsersByRole(roleName string) []*CompoundUser {
	return p.roleToUser.GetAll(roleName)
}
