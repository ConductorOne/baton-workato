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

type collaboratorCache struct {
	client          *client.WorkatoClient
	privilegeToUser map[string][]*CompoundUser
	folderToUser    map[int][]*CompoundUser
	roleToUser      map[string][]*CompoundUser
}

func newCollaboratorCache(workatoClient *client.WorkatoClient) *collaboratorCache {
	return &collaboratorCache{
		client:          workatoClient,
		privilegeToUser: make(map[string][]*CompoundUser),
		folderToUser:    make(map[int][]*CompoundUser),
		roleToUser:      make(map[string][]*CompoundUser),
	}
}

func (p *collaboratorCache) buildCache(ctx context.Context) error {
	l := ctxzap.Extract(ctx)

	l.Info("Building cache for collaborators")

	p.privilegeToUser = make(map[string][]*CompoundUser)
	p.folderToUser = make(map[int][]*CompoundUser)
	p.roleToUser = make(map[string][]*CompoundUser)

	collaborators, err := p.client.GetCollaborators(ctx)
	if err != nil {
		return err
	}

	for _, collaborator := range collaborators {
		collaboratorDetails, err := p.client.GetCollaboratorById(ctx, collaborator.Id)
		if err != nil {
			return err
		}

		compoundUser := &CompoundUser{
			User:       &collaborator,
			UserDetail: collaboratorDetails,
		}

		// Build for privileges
		for keyGroup, values := range collaboratorDetails.Privileges {
			for _, value := range values {
				privilegeKey := workato.PrivilegeId(keyGroup, value)

				p.privilegeToUser[privilegeKey] = append(p.privilegeToUser[privilegeKey], compoundUser)
			}
		}

		// Build for folders
		for _, folderId := range collaboratorDetails.FolderIDs {
			p.folderToUser[folderId] = append(p.folderToUser[folderId], compoundUser)
		}

		// Build for roles
		for _, role := range collaborator.Roles {
			p.roleToUser[role.RoleName] = append(p.roleToUser[role.RoleName], compoundUser)
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
	value, ok := p.privilegeToUser[privilegeKey]
	if !ok {
		return make([]*CompoundUser, 0)
	}

	return value
}

func (p *collaboratorCache) getUsersByFolder(folderId int) []*CompoundUser {
	value, ok := p.folderToUser[folderId]
	if !ok {
		return make([]*CompoundUser, 0)
	}

	return value
}

func (p *collaboratorCache) getUsersByRole(roleName string) []*CompoundUser {
	value, ok := p.roleToUser[roleName]
	if !ok {
		return make([]*CompoundUser, 0)
	}

	return value
}
