package scimtest

import (
	_ "embed"
)

//go:embed user_core.json
var ExampleUserCoreJSON []byte

//go:embed user_withExtension.json
var ExampleUserWithExtensionJSON []byte

//go:embed user_full.json
var ExampleFullUserJSON []byte

var ExampleUserCore = SCIMUser{
	CoreUser: CoreUser{
		Active:      true,
		DisplayName: "Babs Jensen",
		Emails: []Email{
			{
				Value:   "bjensen@example.com",
				Type:    "work",
				Primary: true,
			},
			{
				Value: "babs@jensen.org",
				Type:  "home",
			},
		},
		ExternalID: "701984",
		Groups: []Group{
			{
				Value:   "e9e30dba-f08f-4109-8486-d5c6a331660a",
				Ref:     "https://example.com/v2/Groups/e9e30dba-f08f-4109-8486-d5c6a331660a",
				Display: "Tour Guides",
			},
			{
				Value:   "fc348aa8-3835-40eb-a20b-c726e15c55b5",
				Ref:     "https://example.com/v2/Groups/fc348aa8-3835-40eb-a20b-c726e15c55b5",
				Display: "Employees",
			},
			{
				Value:   "71ddacd2-a8e7-49b8-a5db-ae50d0a5bfd7",
				Ref:     "https://example.com/v2/Groups/71ddacd2-a8e7-49b8-a5db-ae50d0a5bfd7",
				Display: "US Employees",
			},
		},
		ID: "2819c223-7f76-453a-919d-413861904646",
		Name: Name{
			Formatted:       "Ms. Barbara J Jensen, III",
			FamilyName:      "Jensen",
			GivenName:       "Barbara",
			MiddleName:      "Jane",
			HonorificPrefix: "Ms.",
			HonorificSuffix: "III",
		},
		// PreferredLanguage: "en-US", //"preferredLanguage": "en-US",
		// Roles: nil,
		Timezone: "America/Los_Angeles",
		Title:    "Tour Guide",
		UserName: "bjensen@example.com",
		UserType: "Employee",
	},
}

var ExampleUserWithExtension = SCIMUser{
	CoreUser: CoreUser{
		Active:      true,
		DisplayName: "Babs Jensen",
		Emails: []Email{
			{
				Value:   "bjensen@example.com",
				Type:    "work",
				Primary: true,
			},
			{
				Value: "babs@jensen.org",
				Type:  "home",
			},
		},
		ExternalID: "701984",
		Groups: []Group{
			{
				Value:   "e9e30dba-f08f-4109-8486-d5c6a331660a",
				Ref:     "https://example.com/v2/Groups/e9e30dba-f08f-4109-8486-d5c6a331660a",
				Display: "Tour Guides",
			},
			{
				Value:   "fc348aa8-3835-40eb-a20b-c726e15c55b5",
				Ref:     "https://example.com/v2/Groups/fc348aa8-3835-40eb-a20b-c726e15c55b5",
				Display: "Employees",
			},
			{
				Value:   "71ddacd2-a8e7-49b8-a5db-ae50d0a5bfd7",
				Ref:     "https://example.com/v2/Groups/71ddacd2-a8e7-49b8-a5db-ae50d0a5bfd7",
				Display: "US Employees",
			},
		},
		ID: "2819c223-7f76-453a-919d-413861904646",
		Name: Name{
			Formatted:       "Ms. Barbara J Jensen, III",
			FamilyName:      "Jensen",
			GivenName:       "Barbara",
			MiddleName:      "Jane",
			HonorificPrefix: "Ms.",
			HonorificSuffix: "III",
		},
		// PreferredLanguage: "en-US", //"preferredLanguage": "en-US",
		Timezone: "America/Los_Angeles",
		Title:    "Tour Guide",
		UserName: "bjensen@example.com",
		UserType: "Employee",
	},
}
