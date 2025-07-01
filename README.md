[scimprotocol (repo name)] is an impelmentation of SCIM (system for cross domain identity management) RFC 6742/3/4, 
designed for the client side of SCIM, and receive provision requests from the server side of SCIM (Identity Providers).
It only implements the SCIM protocol, a full server also needs a data persistence layer and HTTP service routing.  


## Usage 
1. Define SCIM Resource
	```go
	func init() {
		scimtag.BuildAllSCIMCharacsCache(SCIMUser{}, SCIMGroup{})
	}

	type SCIMUser struct {
		scimprotocol.SCIMResourceMarker
		UserID   *uuid.User
		CoreUser `scim:"urn:ietf:params:scim:schemas:core:2.0:User"` // core schema required to be the first field with a scim tag
		Meta     SCIMMeta                                            `scim:"meta,ignoreUnmarshal,returned=always"`
	}

	var _ scimprotocol.Resource = SCIMUser{}

	type CoreUser struct {
		Active            bool          `scim:"active,returned=keepEmpty"`
		DisplayName       string        `scim:"displayName,caseExact"`
		Emails            []Email       `scim:"emails"`
		Entitlements      []Entitlement `scim:"entitlements"`
		ExternalID        string        `scim:"externalId"`
		Groups            []Group       `scim:"groups,mutability=readOnly"`
		ID                string        `scim:"id,returned=always,ignoreUnmarshal"`
		Name              Name          `scim:"name"`
		PreferredLanguage string        `scim:"preferredLanguage"`
		Roles             []Role        `scim:"roles"`
		Timezone          string        `scim:"timezone"`
		Title             string        `scim:"title"`
		UserName          string        `scim:"userName,required"`
		UserType          string        `scim:"userType"`
	}

	type Email struct {
		Value   string `scim:"value"`
		Type    string `scim:"type,canonicalValues=work"` // azure only allow `work`
		Primary bool   `scim:"primary,returned=keepEmpty"`
	}

	type SCIMMeta struct {
		Created      time.Time `json:"created" scim:"created,returned=always"` // SCIM characteristics need specify at all level
		LastModified time.Time `json:"lastModified" scim:"lastModified,returned=always"`
	}
	```
	- SCIM resources like User and Group must implement the `Resource` interface (embed `SCIMResourceMarker`).
	- When defining SCIM resources, the data type of sub-attributes should be unique if they have different SCIM tag contents, you can use type aliases. [Here is Why](./scimtag/cache.go)
	- Multi-Value attributes, like email, checks duplicate on whole object by default. Optional customized comperation by implement interface `MultiValueElement`.
	- Define scim tag for scim characteristics
		- first positions of SCIM tag is name of the attribute, tag attributes seperated by ','.
		- if boolean then bool or !bool can represent true or false to the variable bool.
		- if you want empty cannonical values shows in schemas, then use `cannonicalValus= `.
		- please check out characteristics.go for all the supported tags.

2. Create SCIM endpoint
	- Use [scimprotocol (repo name)] to create a http server:
		- Use functions in `handlerhelper.go` (Or do something similar) with your persistency later to create a http server
	- Use `scimmarshal.go` to marshal/unmarshal for the endpoint 


## Marshal&Unmarshal
[scimprotocol (repo name)] got it's own [SCIM marshal](scimmarshal.go) with scim tag `scim` and support customization
```
	type SCIMMarshaler interface {
		MarshalSCIM() ([]byte, error)
	}

	type SCIMUnmarshaler interface {
		UnmarshalSCIM([]byte) error
	}
```
Attribute Characteristics `returned` controls the marshal
- returned=default will omit empty
- returned=keepEmpty will keep empty
- returned=request will must marshal when it's been requested
- returned=never will never marshal the field even selected
- returned=always will alway marshal the field even not selected

why we need `keepEmpty` we could just return all supported attributes. But Azure will report error when some attribute not support in Azure, event it's empty. 
So to increase compatibility, since we are not only support Azure, I added the extra return value.

Attribute Characteristics `required` and `ignoreUnmarshal` (used on [SCIMMeta](./exampletest/example_models.go)) controls the unmarshal


## Code
This scim protocol implementation got several pieces
1. [parser](./parser.go) - parse input query contains filter or patch with path.
2. [filter](./filter.go) - after parse we need evaluate a resource, like a user, to check if it's pass the filter or not.
3. [patch](./patch.go) - for patch it need walk down the path along with filter to the target value and modify it.
4. [marshal](./scimmarshal.go) - marshal and unmarshal SCIM object with features like checking schema, select attribute and characteristics.
	- customized SCIM marshal by implemented SCIMMarshaler.
5. [scim tag](./scimtag/) - include SCIM characteristics and tag cache.
7. [handlerhelper](handlerhelper.go) - helper functions to easily interact with database/storage layer for those hander functions when build SCIM http server.

## Azure tweaks
1. Azure use add patch with filter in the path to add new element for multi-value attributes, 
	So I did some hacky way to support simple such action. keyword `azureFilterAdd`
2. Azure put path in value section for patch, like `value: {"name.familyName": "Unua"}`, 
	so I added support for this one. (it's not very hacky so no special keyword for it)

## Note
1. For `schemas` since `name` and `description` is optional and not important so I just skip them
2. test command: `make backend-integration-test BACKEND_TEST="scim/..."`

## Improvement:
- [ ] IMP-1. support bulk
- [ ] IMP-2. Improve return 'requested'?
- [ ] IMP-3. patch on default 'value', like `{path:email, op:add, value:"e@email.com}"`  only support string for now. 
- [ ] IMP-4. add support for number?
- [ ] IMP-5. support for UUID as type?

