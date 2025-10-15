package scimprotocol

import (
	"net/http"

	"github.com/muir/nvelope"
)

type EndpointSCIMIDAndResourceID struct {
	SCIMID     string `nvelope:"path,name=scimID"`
	ResourceID string `nvelope:"path,name=resourceID"`
}

type EndpointSCIMID struct {
	SCIMID string `nvelope:"path,name=scimID"`
}

type Server interface {
	// Authorization middleware for SCIM requests
	Authorization(inner func() error, r *http.Request) error

	// GetResourceTypes returns the list of supported resource types
	GetResourceTypes() []ResourceType

	// Resource handlers for individual resource operations
	GetResourceHandler(r *http.Request, resourceType ResourceType, params EndpointSCIMIDAndResourceID) (nvelope.Response, error)
	GetResourceListHandler(r *http.Request, resourceType ResourceType, params EndpointSCIMID) (nvelope.Response, error)
	PostResourceHandler(r *http.Request, resourceType ResourceType, params EndpointSCIMID) (nvelope.Response, error)
	UpdateResourceHandler(r *http.Request, resourceType ResourceType, params EndpointSCIMIDAndResourceID) (nvelope.Response, error)
	PatchResourceHandler(r *http.Request, resourceType ResourceType, params EndpointSCIMIDAndResourceID) (nvelope.Response, error)
	DeleteResourceHandler(r *http.Request, resourceType ResourceType, params EndpointSCIMIDAndResourceID) (nvelope.Response, error)

	// Bulk operations handler
	BulkHandler() (nvelope.Response, error)

	// SCIM metadata handlers
	GetResourceTypesHandler() (nvelope.Response, error)
	SchemasHandler() (nvelope.Response, error)
	GetServiceProviderConfigHandler(r *http.Request, w http.ResponseWriter) (nvelope.Response, error)
}
