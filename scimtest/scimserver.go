package scimtest

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/memsql/errors"
	"github.com/muir/nvelope"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/util"
)

type Server struct {
	trace         util.Trace
	storage       *Storage
	ResourceTypes []scimprotocol.ResourceType
	config        scimprotocol.Config
}

const DummyToken = "dummyToken"

func NewServer(trace util.Trace, storage *Storage) Server {
	return Server{
		trace:   trace,
		storage: storage,
		ResourceTypes: []scimprotocol.ResourceType{
			{
				ID:                 "User",
				Name:               "User",
				Endpoint:           "/Users",
				ResourceObjectType: reflect.TypeOf(SCIMUser{}), // no need content
			},
			{
				ID:                 "Group",
				Name:               "Group",
				Endpoint:           "/Groups",
				ResourceObjectType: reflect.TypeOf(SCIMGroup{}), // no need content
			},
		},
		config: scimprotocol.Config{
			DefaultPageSize:    100,
			PatchSupported:     true,
			BulkSupported:      false,
			BulkMaxOperations:  1000,
			BulkMaxPayloadSize: 1048576,
			FilterSupported:    true,
			FilterMaxResult:    200,
			ChangePassword:     false,
			SortSupported:      false,
			EtagSupported:      false,
			AuthenticationSchemas: []scimprotocol.AuthenticationScheme{
				{
					Name:        "OAuth Bearer Token",
					Description: "Authentication scheme using the OAuth Bearer Token Standard",
					SpecURI:     "http://www.rfc-editor.org/info/rfc6750",
					// TODO: add doc url
					DocumentationURI: "http://example.com/help/oauth.html",
					Type:             scimprotocol.AuthTypeOauthBearerToken,
					Primary:          true,
				},
			},
		},
	}
}

func (h Server) Authorization(inner func() error, r *http.Request) error {
	// check secret
	secret := r.Header.Get("Authorization")
	if secret == "" {
		return scimerror.NewSCIMErr(http.StatusUnauthorized, errors.Errorf("Authorization header not found in (%s %s) request", r.Method, r.URL))
	}
	parts := strings.Split(secret, " ")
	if parts[0] != "Bearer" || len(parts) != 2 {
		// Okta SCIM 2.0 SPEC requires 401 (not 403) for a malformed token.
		return scimerror.NewSCIMErr(http.StatusUnauthorized, errors.Errorf("Authorization header in (%s %s) request is not a Bearer token", r.Method, r.URL))
	}
	secretWOPrefix := parts[1]
	if DummyToken != secretWOPrefix {
		return scimerror.NewSCIMErr(http.StatusUnauthorized, errors.Errorf("invalid SCIM API key"))
	}
	return inner()
}

func (h Server) GetResourceTypes() []scimprotocol.ResourceType {
	return h.ResourceTypes
}

func (h Server) GetResourceHandler(r *http.Request, resourceType scimprotocol.ResourceType, params scimprotocol.EndpointSCIMIDAndResourceID) (nvelope.Response, error) {
	switch resourceType.Endpoint {
	case "/Users":
		return scimprotocol.GetResourceHelper(r, params.SCIMID, params.ResourceID, h.storage.GetSCIMUser)
	case "/Groups":
		return scimprotocol.GetResourceHelper(r, params.SCIMID, params.ResourceID, h.storage.GetSCIMGroup)
	default:
		return nil, errors.Errorf("unsupported resource %s", resourceType.Endpoint)
	}
}

func (h Server) GetResourceListHandler(r *http.Request, resourceType scimprotocol.ResourceType, params scimprotocol.EndpointSCIMID) (nvelope.Response, error) {
	switch resourceType.Endpoint {
	case "/Users":
		return scimprotocol.GetListResourceHelper(r, h.trace, h.config.DefaultPageSize, params.SCIMID, h.storage.GetAllSCIMUser)
	case "/Groups":
		return scimprotocol.GetListResourceHelper(r, h.trace, h.config.DefaultPageSize, params.SCIMID, h.storage.GetAllSCIMGroup)
	default:
		return nil, errors.Errorf("unsupported resource %s", resourceType.Endpoint)
	}
}

func (h Server) PostResourceHandler(r *http.Request, resourceType scimprotocol.ResourceType, params scimprotocol.EndpointSCIMID) (nvelope.Response, error) {
	switch resourceType.Endpoint {
	case "/Users":
		return scimprotocol.CreateResourceHelper(r, params.SCIMID, h.storage.CreateSCIMUser)
	case "/Groups":
		return scimprotocol.CreateResourceHelper(r, params.SCIMID, h.storage.CreateSCIMGroup)
	default:
		return nil, errors.Errorf("unsupported resource %s", resourceType.Endpoint)
	}
}

func (h Server) UpdateResourceHandler(r *http.Request, resourceType scimprotocol.ResourceType, params scimprotocol.EndpointSCIMIDAndResourceID) (nvelope.Response, error) {
	switch resourceType.Endpoint {
	case "/Users":
		return scimprotocol.UpdateResourceHelper(r, params.SCIMID, params.ResourceID, h.storage.UpdateSCIMUser)
	case "/Groups":
		return scimprotocol.UpdateResourceHelper(r, params.SCIMID, params.ResourceID, h.storage.UpdateSCIMGroup)
	default:
		return nil, errors.Errorf("unsupported resource %s", resourceType.Endpoint)
	}
}

func (h Server) PatchResourceHandler(r *http.Request, resourceType scimprotocol.ResourceType, params scimprotocol.EndpointSCIMIDAndResourceID) (nvelope.Response, error) {
	switch resourceType.Endpoint {
	case "/Users":
		return scimprotocol.PatchResourceHelper(r, params.SCIMID, params.ResourceID, h.storage.GetSCIMUser, h.storage.UpdateSCIMUser)
	case "/Groups":
		return scimprotocol.PatchResourceHelper(r, params.SCIMID, params.ResourceID, h.storage.GetSCIMGroup, h.storage.UpdateSCIMGroup)
	default:
		return nil, errors.Errorf("unsupported resource %s", resourceType.Endpoint)
	}
}

func (h Server) DeleteResourceHandler(r *http.Request, resourceType scimprotocol.ResourceType, params scimprotocol.EndpointSCIMIDAndResourceID) (nvelope.Response, error) {
	switch resourceType.Endpoint {
	case "/Users":
		return nil, h.storage.DeleteSCIMUser(r.Context(), params.SCIMID, params.ResourceID)
	case "/Groups":
		return nil, h.storage.DeleteSCIMGroup(r.Context(), params.SCIMID, params.ResourceID)
	default:
		return nil, errors.Errorf("unsupported resource %s", resourceType.Endpoint)
	}
}

func (h Server) BulkHandler() (nvelope.Response, error) {
	return nil, errors.Errorf("not implement")
}

func (h Server) GetResourceTypesHandler() (nvelope.Response, error) {
	return h.ResourceTypes, nil
}

func (h Server) SchemasHandler() (resp nvelope.Response, err error) {
	return scimprotocol.GetSchemasHelper(h.ResourceTypes)
}

func (h Server) GetServiceProviderConfigHandler(r *http.Request, w http.ResponseWriter) (nvelope.Response, error) {
	return h.config.MarshalSCIM(r.URL.String())
}
