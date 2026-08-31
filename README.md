SingleStore-Lab/scim is an implementations of SCIM (system for cross domain identity management) RFC6742/RFC6743/RFC6744. It designed to receive SCIM provision requests which typically is from identity providers. It implements the SCIM protocol with router and server interface. Example usage at `/scimtest` folder. 


## Usage 
1. Define SCIM Resources
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
	- When defining SCIM resources, the type need to be unique. [Here is Why](./scimtag/cache.go)
	- Multi-Value attributes, like email, checks duplicate on whole object by default. You can customize comparation by implementing interface `MultiValueElement`.
	- Define scim tags for scim characteristics
		- First position of SCIM tag is name of the attribute. Tag attributes separated by ','.
		- `bool` or `!bool` in SCIM tag can represent true or false.
		- If you want empty canonical values shows in schemas, then use `cannonicalValus= `.
		- Please check out [characteristics.go](./scimtag/characteristics.go) for all the supported tags.

2. Create SCIM endpoints

	[scimrouter](router.go) is available to use, check [example](./scimtest/scimserver.go) in scimtest folder.
	
	You can also create your own router and server with helper function below. 
	- Use SingleStore-Lab/scim to create a http server:
		- Use functions in `handlerhelper.go` (Or do something similar) with your persistency later to create a http server
	- Use `scimmarshal.go` to marshal/unmarshal when the data goes through endpoints 


## Marshal&Unmarshal
SingleStore-Lab/scim got it's own [SCIM marshal](scimmarshal.go) with scim [tag](./scimtag/) `scim`.

Attribute [Characteristics](./scimtag/characteristics.go) `returned` controls the marshal
- returned=default will omit empty
- returned=keepEmpty will return empty values
- returned=request will must marshal when it's been requested
- returned=never will never marshal the field even selected
- returned=always will alway marshal the field even not selected

Attribute [Characteristics](./scimtag/characteristics.go) `required` and `ignoreUnmarshal` controls the unmarshal

NOTE: Why we need `keepEmpty` while it's not in the RFC standards? Because we need ability to hide some empty attributes while keeping some necessary attributes to support multiple identity providers.

SingleStore-Lab/scim supports customization marshal, however, if you use SCIMMarshaler/SCIMUnmarshaler, then filter and patch will not works on that resource.
```
	type SCIMMarshaler interface {
		MarshalSCIM() ([]byte, error)
	}

	type SCIMUnmarshaler interface {
		UnmarshalSCIM([]byte) error
	}
```
The `PrimaryDataType` interface helps support customized primary data type not in the rfc, like `ID` to support UUID type. [Example: TestMarshalObject](./scimmarshal_test.go) 

You could also use string for `ID` and convert it to your type at outside of this Library. 
```
type PrimaryDataType interface {
	SCIMCompareValue(op string, stringValue string, azureAdd bool) (bool, error)
}
```



## Code
1. [parser](./parser.go) - parse input query contains filter or patch with path.
2. [filter](./filter.go) - after parse we need evaluate a resource, like a user or a group, to check if it's passes the filter or not.
3. [patch](./patch.go) - patch needs to walk down the path along with filter to the target value and modifies it.
4. [marshal](./scimmarshal.go) - marshal and unmarshal SCIM object with characteristics and support features like checking schema, select attributes.
	- customized SCIM marshal by implementing SCIMMarshaler.
5. [scim tag](./scimtag/) - include SCIM characteristics and tag cache.
7. [handlerhelper](handlerhelper.go) - helper functions helps easily interact with database/storage layer for those handler functions when building SCIM http server.

## Azure tweaks
Azure uses filter in a patch to add new element for multi-value attributes. Like if non of the element can pass filter then it will add one with the value in the patch. Use `azureFilterAdd` to trigger support adding elements to multi-value attributes with filters.


## Note
This repo does not contains `name` and `description` in `schemas` because they are optional in RFC.

## Improvement:
- [ ] IMP-1. support bulk
- [ ] IMP-2. Improve return 'requested'?
- [ ] IMP-3. add support for number?

