package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/session"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type collaboratorCache struct {
	client *client.WorkatoClient
}

func newCollaboratorCache(workatoClient *client.WorkatoClient) *collaboratorCache {
	return &collaboratorCache{
		client: workatoClient,
	}
}

const (
	collaboratorNamespace = "collaborator"
)

func (c *collaboratorCache) setCollaboratorsCache(ctx context.Context, sessionStorage sessions.SessionStore, collaborators []*client.Collaborator) error {
	collaboratorMap := make(map[string]*client.Collaborator)
	for _, collaborator := range collaborators {
		collaboratorMap[strconv.Itoa(collaborator.Id)] = collaborator
	}
	err := session.SetManyJSON(ctx, sessionStorage, collaboratorMap, sessions.WithPrefix(collaboratorNamespace))
	if err != nil {
		return fmt.Errorf("failed to set collaborators cache in session storage: %w", err)
	}
	return nil
}

// getCollaborator gets a collaborator from the cache by id.
// It is assumed that the cache is populated when listing resources and is only used when listing grants.
func (c *collaboratorCache) getCollaborator(ctx context.Context, sessionStorage sessions.SessionStore, collaboratorId string) (*client.Collaborator, error) {
	collaborator, found, err := session.GetJSON[client.Collaborator](ctx, sessionStorage, collaboratorId, sessions.WithPrefix(collaboratorNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to get collaborator from session storage: %w", err)
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "collaborator not found")
	}
	return &collaborator, nil
}
