[![Go Reference](https://pkg.go.dev/badge/github.com/singlestore-labs/scim.svg)](https://pkg.go.dev/github.com/singlestore-labs/scim)

# SingleStore-Lab/scim — SCIM 2.0 library for Go

A Go (Golang) library for building a **SCIM 2.0** server that receives identity **provisioning** requests from identity providers (IdPs) such as **Microsoft Entra ID (Azure AD)** and **Okta**.

SCIM is the **System for Cross-domain Identity Management**. This package implements the SCIM protocol and core schema so you can expose User and Group endpoints, parse filters and PATCH paths, and marshal SCIM JSON. Attribute behavior follows **RFC 7643 SCIM characteristics** declared as `scim` struct tags.

- **RFC 7642** — SCIM definitions, overview, and requirements
- **RFC 7643** — SCIM core schema (User, Group, schemas, resource types)
- **RFC 7644** — SCIM protocol (HTTP API, filter, PATCH, pagination, attribute selection)

Import: `github.com/singlestore-labs/scim`  
Package: `scimprotocol`  
API docs: [pkg.go.dev/github.com/singlestore-labs/scim](https://pkg.go.dev/github.com/singlestore-labs/scim)

The library does not include persistence. To use it, you must supply a persistence layer meeting the API.

A working in-memory SCIM HTTP server is in [`scimtest`](./scimtest).

## Features

- **SCIM characteristics** (RFC 7643 §7): `required`, `caseExact`, `mutability`, `returned`, `uniqueness`, `canonicalValues`...
- SCIM 2.0 **Users** and **Groups** (and other resource types you define)
- HTTP router for SCIM endpoints (`/Users`, `/Groups`, `/Schemas`, `/ResourceTypes`, `/ServiceProviderConfig`)
- Filter parser and evaluation (`userName eq "bjensen"`, nested multi-value filters)
- PATCH add / remove / replace, including Azure-style filter-add on multi-value attributes
- SCIM marshal / unmarshal driven by SCIM characteristics (`scim` struct tags)
- Handler helpers that sit in front of your own database or storage layer

## SCIM characteristics

[RFC 7643 §7](https://datatracker.ietf.org/doc/html/rfc7643#section-7) attribute characteristics are declared on every field with a `scim` tag. They control marshal, unmarshal, filter, PATCH, and `/Schemas` output. See [`scimtag/characteristics.go`](./scimtag/characteristics.go).

Example: `` `scim:"userName,returned=always,required"` ``

| Characteristic | Tag | Used for |
| --- | --- | --- |
| name | first tag token | JSON name, filter/PATCH path, schemas |
| required | `required` / `!required` | unmarshal validation |
| caseExact | `caseExact` / `!caseExact` | filter comparison |
| mutability | `mutability=` | PATCH / update (`readWrite`, `readOnly`, `immutable`, `writeOnly`) |
| returned | `returned=` | marshal / `attributes=` selection (`default`, `keepEmpty`, `always`, `never`, `request`) |
| uniqueness | `uniqueness=` | scope of the unique value: `none`, `server`, or `global` |
| canonicalValues | `canonicalValues=` | allowed values in schemas (e.g. email `type`: `work`) |
| referenceTypes | `referenceTypes=` | resource types a `$ref` may point to — e.g. a User's `groups.$ref` with `referenceTypes=User Group` |
| ignoreUnmarshal | `ignoreUnmarshal` | skip unmarshal (`id`, `meta`) |

`returned=keepEmpty` is an extension (emitted as `default` in schemas) so some attributes can stay visible while others omit empty values — useful for Azure AD / Entra ID and other IdPs.

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
	- Define `scim` tags for [SCIM characteristics](#scim-characteristics)
		- First position of the tag is the attribute name. Characteristics are separated by `,`.
		- `bool` or `!bool` in the tag is true or false (`required`, `caseExact`, `ignoreUnmarshal`).
		- Empty canonical values in schemas: `canonicalValues= `.
		- Characteristics must be set on nested structs as well (see `SCIMMeta` above).

2. Create SCIM endpoints

	[scimrouter](router.go) is available to use, check [example](./scimtest/scimserver.go) in scimtest folder.
	
	You can also create your own router and server with helper function below. 
	- Use this library to create a HTTP server:
		- Use functions in `handlerhelper.go` (or do something similar) with your persistency layer to create a HTTP server
	- Use `scimmarshal.go` to marshal/unmarshal when the data goes through endpoints 


## Marshal&Unmarshal
This library has its own [SCIM marshal](scimmarshal.go) with scim [tag](./scimtag/) `scim`. Marshal and unmarshal follow [SCIM characteristics](#scim-characteristics).

`returned` controls marshal:
- `returned=default` omit empty
- `returned=keepEmpty` include empty values
- `returned=request` marshal only when selected
- `returned=never` never marshal, even if selected
- `returned=always` always marshal, even if not selected

`required` and `ignoreUnmarshal` control unmarshal.

This library supports customization marshal, however, if you use SCIMMarshaler/SCIMUnmarshaler, then filter and patch will not works on that resource.
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

## Okta SCIM compatibility test

The end-to-end test replays Okta's official
[SCIM 2.0 SPEC export](https://developer.okta.com/standards/SCIM/SCIMFiles/Okta-SCIM-20-SPEC-Test.json)
against the in-memory example server. It runs locally without a Runscope or
BlazeMeter account:

```sh
go test ./e2e -run TestOktaSCIM20Spec -v
```

GitHub Actions starts `cmd/scimtestd` as a separate local process before
running the same test. To test another Runscope API Monitoring export or a
different server:

```sh
RUNSCOPE_SUITE=/path/to/export.json \
RUNSCOPE_VARIABLES='{"newVariable":"value"}' \
SCIM_BASE_URL=http://127.0.0.1:18080/scim/v2 \
SCIM_AUTH='Bearer dummyToken' \
go test ./e2e -run TestOktaSCIM20Spec -v
```

`RUNSCOPE_VARIABLES` is optional and overrides the built-in Okta initial
variables. This allows another export to provide additional placeholders
without changing the runner.

The reusable executor in `e2e/runscope` supports request and pause steps,
variable substitution and extraction, and the assertion types used by Okta's
current export. Unsupported future step, assertion, or script types fail with
an explicit error instead of being silently ignored.


## Note
This repo does not contains `name` and `description` in `schemas` because they are optional in RFC.
