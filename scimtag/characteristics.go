package scimtag

import "github.com/memsql/errors"

const TagName = "scim"

// Characteristics (Characs) contains information about each field of Resource (Users, Groups),
// which controls how to handle this field/attribute.
// RFC: https://datatracker.ietf.org/doc/html/rfc7643#section-7
//
// Characteristics MUST be specified by 'scim' tag in Resource struct and will parse to this struct,
// not specified in tag will be default value,
// like `scim:"userName,returned=always,required"`:
//
//	{
//		Name: "userName",
//		Returned: "always",
//		Required: true,
//		CaseExact: false, // default value
//		....
//	}
//
// Those information controls how to handle this field or attribute.
// For example 'returned' controls marshal behavior.
type Characteristics struct {
	Name            string     `pt:"0"` // MUST set, else it gets value from other's
	Required        bool       `pt:"required"`
	CaseExact       bool       `pt:"caseExact"`
	Mutability      Mutability `pt:"mutability"`                  // default 'readWrite'
	Returned        Returned   `pt:"returned"`                    // default 'default'
	Uniqueness      Uniqueness `pt:"uniqueness"`                  // none (default), server, global
	CanonicalValues []string   `pt:"canonicalValues,split=space"` // like for email type, 'home', 'work','others'
	ReferenceTypes  []string   `pt:"referenceTypes,split=space"`  // like for groups's $ref, 'User', 'Group'
	// special
	IgnoreUnmarshal bool   `pt:"ignoreUnmarshal"` // special for 'meta'
	SQL             string `pt:"sql"`
}

//go:generate enumer -type=Uniqueness -text -transform=lower
type Uniqueness int

// RFC values: none server global
const (
	None Uniqueness = iota
	Server
	Global
)

//go:generate enumer -type=Mutability -text -transform=title-lower
type Mutability int

const (
	ReadWrite Mutability = iota
	ReadOnly
	Immutable
	WriteOnly
)

// Returned is an characteristics of attributes.
// It controls how SCIM returned, in this implementation it controls marshal
type Returned int

const (
	Default   Returned = iota // will omit empty when marshal
	KeepEmpty                 // will marshal empty value (It's not in RFC and will marshal to 'default' in schema)
	Always                    // will alway marshal the field (even not request)
	Never                     // will never marshal the field (even requested)
	Request                   // will marshal when it's been requested
)

var returnedToString = map[Returned]string{
	Default:   "default",
	KeepEmpty: "default", // only different in tag, output should be `default`
	Always:    "always",
	Never:     "never",
	Request:   "request",
}

var stringToReturnd = map[string]Returned{
	"default":   Default,
	"always":    Always,
	"never":     Never,
	"request":   Request,
	"keepEmpty": KeepEmpty,
}

func (r Returned) String() string {
	if str, ok := returnedToString[r]; ok {
		return str
	}
	return ""
}

func (r Returned) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r *Returned) UnmarshalText(text []byte) error {
	if returned, ok := stringToReturnd[string(text)]; ok {
		*r = returned
	} else {
		return errors.Errorf("could not unmarshal text %s", string(text))
	}
	return nil
}
