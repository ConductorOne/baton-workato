package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

// The user resource type is for all user objects from the database.
var collaboratorResourceType = &v2.ResourceType{
	Id:          "collaborator",
	DisplayName: "Collaborator",
	Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_USER},
}
