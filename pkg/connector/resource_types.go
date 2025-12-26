package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

// The user resource type is for all user objects from the database.
var collaboratorResourceType = &v2.ResourceType{
	Id:          "collaborator",
	DisplayName: "Collaborator",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
}

var privilegeResourceType = &v2.ResourceType{
	Id:          "privilege",
	DisplayName: "Privilege",
	Annotations: annotations.New(&v2.SkipGrants{}),
}

var roleResourceType = &v2.ResourceType{
	Id:          "role",
	DisplayName: "Roles",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
}

var folderResourceType = &v2.ResourceType{
	Id:          "folder",
	DisplayName: "Folder",
}

var projectResourceType = &v2.ResourceType{
	Id:          "project",
	DisplayName: "Project",
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}

var environmentResourceType = &v2.ResourceType{
	Id:          "environment",
	DisplayName: "Environment",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	Annotations: annotations.New(&v2.SkipEntitlementsAndGrants{}),
}
