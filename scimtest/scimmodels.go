package scimtest

import (
	"time"

	"github.com/google/uuid"
	"github.com/singlestore-labs/generic"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimtag"
)

func init() {
	scimtag.BuildAllSCIMCharacsCache(SCIMUser{}, SCIMGroup{})
}

// type UserID string
type SCIMID uuid.UUID
type UserID uuid.UUID
type TeamID string

type SCIMResource interface {
	scimprotocol.Resource
	Copy() SCIMResource
}
type SCIMUserID string

type SCIMGroupID string

type SCIMUser struct {
	scimprotocol.SCIMResourceMarker
	UserID   *UserID                                             // non-scim tagged field will not be considered in SCIM related operations like marshal/filter/patch
	CoreUser `scim:"urn:ietf:params:scim:schemas:core:2.0:User"` // core schema required to be the first field with a scim tag
	Meta     SCIMMeta                                            `scim:"meta,ignoreUnmarshal,returned=always"`
}

func (su SCIMUser) Copy() SCIMUser {
	newSCIMUser := su
	newSCIMUser.Entitlements = generic.CopySlice[Entitlement](su.Entitlements)
	newSCIMUser.Emails = generic.CopySlice[Email](su.Emails)
	newSCIMUser.Groups = generic.CopySlice[Group](su.Groups)
	newSCIMUser.Roles = generic.CopySlice[Role](su.Roles)
	return newSCIMUser
}

var _ scimprotocol.Resource = SCIMUser{}

type CoreUser struct {
	Active            bool          `scim:"active,returned=keepEmpty"`
	DisplayName       string        `scim:"displayName,caseExact"`
	Emails            []Email       `scim:"emails"`
	Entitlements      []Entitlement `scim:"entitlements"`
	ExternalID        string        `scim:"externalId"`
	Groups            []Group       `scim:"groups,mutability=readOnly"`
	ID                string        `scim:"id,mutability=readOnly,returned=always,ignoreUnmarshal"`
	Name              Name          `scim:"name"`
	PreferredLanguage string        `scim:"preferredLanguage"`
	Roles             []Role        `scim:"roles"`
	Timezone          string        `scim:"timezone"`
	Title             string        `scim:"title"`
	UserName          string        `scim:"userName,required"`
	UserType          string        `scim:"userType"`
}

type SCIMMeta struct {
	// json tag here is for test only
	Created      time.Time `json:"created"      scim:"created,returned=always"` // characteristics need specify at all level
	LastModified time.Time `json:"lastModified" scim:"lastModified,returned=always"`
}

type Email struct {
	Value   string `json:"value"   scim:"value"`
	Type    string `json:"type"    scim:"type,canonicalValues=work"` // azure only allow work
	Primary bool   `json:"primary" scim:"primary,returned=keepEmpty"`
}

func (e *Email) IsEmpty() bool {
	return e != nil && e.Value == "" && e.Type == "" && !e.Primary
}

type Entitlement struct {
	Value   string `json:"value"   scim:"value"`
	Display string `json:"display" scim:"display"`
	Type    string `json:"type"    scim:"type,canonicalValues= "` // special canonicalValues to return empty [] in schemas
	Primary bool   `json:"primary" scim:"primary"`
}

func (e *Entitlement) IsEmpty() bool {
	return e != nil && e.Value == "" && e.Display == "" && e.Type == "" && !e.Primary
}

type Role struct {
	Value   string `json:"value"   scim:"value"`
	Display string `json:"display" scim:"display"`
	Type    string `json:"type"    scim:"type,canonicalValues= "` // special canonicalValues to return empty [] in schemas
	Primary bool   `json:"primary" scim:"primary"`
}

func (r *Role) IsEmpty() bool {
	return r != nil && r.Value == "" && r.Display == "" && r.Type == "" && !r.Primary
}

type Group struct {
	Value   string `scim:"value"`
	Display string `scim:"display"`
	Type    string `scim:"type,canonicalValues= "`         // special canonicalValues to return empty [] in schemas, azure support
	Ref     string `scim:"$ref,referenceTypes=User Group"` // string array in tag separate by space
}

type Name struct {
	Formatted       string `json:"formatted"       scim:"formatted"`
	FamilyName      string `json:"familyName"      scim:"familyName"`
	GivenName       string `json:"givenName"       scim:"givenName"`
	MiddleName      string `json:"middleName"      scim:"middleName"`
	HonorificPrefix string `json:"honorificPrefix" scim:"honorificPrefix"`
	HonorificSuffix string `json:"honorificSuffix" scim:"honorificSuffix"`
}

func (n *Name) IsEmpty() bool {
	return n != nil && n.Formatted == "" && n.FamilyName == "" && n.GivenName == "" &&
		n.MiddleName == "" && n.HonorificPrefix == "" && n.HonorificSuffix == ""
}

type SCIMGroup struct {
	scimprotocol.SCIMResourceMarker
	TeamID    TeamID
	CoreGroup `scim:"urn:ietf:params:scim:schemas:core:2.0:Group"`
	Meta      SCIMMeta `scim:"meta,ignoreUnmarshal"`
}

var _ scimprotocol.Resource = SCIMGroup{}

type CoreGroup struct {
	ID          string            `scim:"id,mutability=readOnly,returned=always,ignoreUnmarshal"`
	ExternalID  string            `scim:"externalId"`
	DisplayName string            `scim:"displayName"`
	Members     []SCIMGroupMember `scim:"members"`
}

type SCIMGroupMember struct {
	Value   string `scim:"value,required"`
	Ref     string `scim:"$ref"`
	Display string `scim:"display"`
	Type    string `scim:"type,canonicalValues= "`
}

type SCIMUserDBJSONFields struct {
	Name         []byte
	Emails       []byte
	Entitlements []byte
	Groups       []byte
	Roles        []byte
}
