// Package scimprotocol implements SCIM 2.0 (RFC 7643 and RFC 7644) for
// receiving provisioning requests from identity providers.
//
// The import path is github.com/singlestore-labs/scim.
//
// # Resources
//
// Define User, Group, and other resources by embedding [SCIMResourceMarker]
// and tagging fields with scim struct tags. See
// [github.com/singlestore-labs/scim/scimtag] for supported characteristics.
// Call [github.com/singlestore-labs/scim/scimtag.BuildAllSCIMCharacsCache]
// once at startup for each resource type.
//
// # HTTP endpoints
//
// Implement [Server] and pass it to [SCIMRouter], or build your own handlers
// with helpers such as [GetResourceHelper] and [GetListResourceHelper].
// Marshal and unmarshal with [Marshal] and [Unmarshal]; they honor scim tags
// for attribute selection, required fields, and empty-value handling.
//
// A complete in-memory example lives in the scimtest package.
package scimprotocol
