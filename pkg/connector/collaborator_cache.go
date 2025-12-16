package connector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/conductorone/baton-sdk/pkg/session"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type collaboratorCache struct {
	client *client.WorkatoClient
	env    workato.Environment
}

func newCollaboratorCache(workatoClient *client.WorkatoClient, env workato.Environment) *collaboratorCache {
	return &collaboratorCache{
		client: workatoClient,
		env:    env,
	}
}

const (
	collaboratorNamespace = "collaborator"
)

func (c *collaboratorCache) setCollaboratorsCache(ctx context.Context, sessionStorage sessions.SessionStore, collaborators []client.Collaborator) error {
	collaboratorMap := make(map[string]client.Collaborator)
	for _, collaborator := range collaborators {
		collaboratorMap[strconv.Itoa(collaborator.Id)] = collaborator
	}
	err := session.SetManyJSON(ctx, sessionStorage, collaboratorMap, sessions.WithPrefix(collaboratorNamespace))
	if err != nil {
		return fmt.Errorf("failed to set collaborators cache in session storage: %w", err)
	}
	return nil
}

func (c *collaboratorCache) getCollaborator(ctx context.Context, sessionStorage sessions.SessionStore, collaboratorId string) (client.Collaborator, error) {
	collaborator, found, err := session.GetJSON[client.Collaborator](ctx, sessionStorage, collaboratorId, sessions.WithPrefix(collaboratorNamespace))
	if err != nil {
		return client.Collaborator{}, fmt.Errorf("failed to get collaborator from session storage: %w", err)
	}
	if !found {
		// TODO: Add fallback to get collaborator from API
		return client.Collaborator{}, status.Errorf(codes.NotFound, "collaborator not found")
	}
	return collaborator, nil
}
