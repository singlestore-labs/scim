package scimprotocol

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/alecthomas/participle/v2"
	"github.com/stretchr/testify/require"
)

func TestTokenizePattern(t *testing.T) {
	t.Parallel()
	{
		t.Logf("test string regex pattern: %s", stringPattern)
		re := regexp.MustCompile(fmt.Sprintf("^(?:%s)$", stringPattern))
		require.False(t, re.MatchString(`test`), `testing string pattern for content without no double quote`)
		require.True(t, re.MatchString(`"test"`), `testing string pattern for content with double quote`)
		require.True(t, re.MatchString(`""`), `testing string pattern for empty content but with double quote`)
	}
	{
		t.Logf("test number regex pattern: %s", numberPattern)
		re := regexp.MustCompile(fmt.Sprintf("^(?:%s)$", numberPattern))
		require.False(t, re.MatchString(``), `testing number pattern for empty content`)
		require.True(t, re.MatchString(`123`), `testing number pattern for integer`)
		require.True(t, re.MatchString(`-123`), `testing number pattern for negative integer`)
		require.False(t, re.MatchString(`"123"`), `testing number pattern for integer with double quote`)
		require.True(t, re.MatchString(`123.123`), `testing number pattern for float number`)
		require.True(t, re.MatchString(`-123.123`), `testing number pattern for negative float number`)
		require.False(t, re.MatchString(`123..123`), `testing number pattern for invalid extra .`)
		require.False(t, re.MatchString(`123.12.3`), `testing number pattern for invalid extra . v2`)
	}
}

// TestParseFilterKeywordsAsAttrNames covers RFC 7643 §2.1 ATTRNAME, which allows
// any ALPHA *(nameChar) and therefore collides with every filter keyword.
func TestParseFilterKeywordsAsAttrNames(t *testing.T) {
	t.Parallel()
	cases := []string{
		// attribute names that merely start with a keyword
		`trueName eq "x"`,
		`nullable pr`,
		`falseFlag eq false`,
		`notes co "x"`,
		`android pr`,
		`origin eq "x"`,
		// attribute names that are exactly a keyword
		`true eq true`,
		`null pr`,
		`not pr`,
		`and eq "x"`,
		`or eq "x"`,
		`eq eq "x"`,
	}
	for _, input := range cases {
		_, err := ParseFilter(input)
		require.NoError(t, err, input)
	}
}

func TestParseFilterNotGrouping(t *testing.T) {
	t.Parallel()
	// The ABNF has no SP after "not"; the RFC examples and identity providers
	// include one. Both forms, in any case, must negate the group.
	for _, input := range []string{
		`not (userName pr)`,
		`not(userName pr)`,
		`NOT (userName pr)`,
		`Not(userName pr)`,
	} {
		parsed, err := ParseFilter(input)
		require.NoError(t, err, input)
		ne, ok := parsed.Left.Left.(NotExpression)
		require.True(t, ok, input)
		require.True(t, ne.Not, input)
	}

	// a bare group is the same production with zero "not"
	parsed, err := ParseFilter(`(userName pr)`)
	require.NoError(t, err)
	ne, ok := parsed.Left.Left.(NotExpression)
	require.True(t, ok)
	require.False(t, ne.Not)
}

func TestParseFilterInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		`not`, // FILTER requires attrExp, not a bare path
		`userName`,
		`emails`,
		`emails[userName]`,       // valFilter is a FILTER too
		`not (userName)`,         // so is a grouped FILTER
		`userName eq "x" or not`, // and every operand of a logExp
		`userName eq`,            // compareOp requires a compValue
		`userName pr "x"`,        // 'pr' takes no compValue
		`userName foo "x"`,       // not a compareOp
		`userName eq bjensen`,    // unquoted string is not a compValue
		`not userName pr`,        // 'not' requires a group
		`(userName pr`,           // unbalanced group
		`userName eq "x" and`,    // dangling logical operator
		`emails[type eq "work"`,           // unbalanced value filter
		`emails[type eq "work"].value`,    // valuePath [subAttr] is a PATH, not a FILTER
		`not (emails[type eq "work"].value)`,
		`userName eq "x" or emails[type eq "work"].value`,
		`active eq TRUE`, // JSON true/false/null are lowercase only (RFC 7159)
		`active eq False`,
		`manager eq Null`,
	}
	for _, input := range cases {
		_, err := ParseFilter(input)
		require.Error(t, err, input)
	}
}

func TestParseFilterCompValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Input string
		Want  CompValue
	}{
		{`active eq true`, "true"},
		{`active eq false`, "false"},
		{`manager eq null`, "null"},
		{`count eq 12`, "12"},
		{`count eq -12.5`, "-12.5"},
		{`userName eq "bjensen"`, `"bjensen"`},
	}
	for _, c := range cases {
		parsed, err := ParseFilter(c.Input)
		require.NoError(t, err, c.Input)
		expr, ok := parsed.Left.Left.(Expression)
		require.True(t, ok, c.Input)
		require.Equal(t, c.Want, expr.Value, c.Input)
	}
}

func TestPathParser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Input string
		Want  *Path
	}{
		{
			"",
			nil,
		},
		{
			"userName",
			&Path{
				AttrName: "userName",
			},
		},
		{
			`emails[type eq "work" and value co "@example.com"]`,
			&Path{
				AttrName: "emails",
				Filter: &OrExpression{
					Left: &AndTerm{
						Left: Expression{
							Path: Path{
								AttrName: "type",
							},
							CompareOp: "eq",
							Value:     `"work"`,
						},
						Right: []Expr{
							Expression{
								Path: Path{
									AttrName: "value",
								},
								CompareOp: "co",
								Value:     `"@example.com"`,
							},
						},
					},
				},
			},
		},
		{
			"name.familyName",
			&Path{
				AttrName:    "name",
				SubAttrName: "familyName",
				Filter:      nil,
			},
		},
		{
			"urn:ietf:params:scim:schemas:core:2.0:User:userName",
			&Path{
				URI:      "urn:ietf:params:scim:schemas:core:2.0:User",
				AttrName: "userName",
			},
		},
		{
			"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:employeeNumber",
			&Path{
				URI:      "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User",
				AttrName: "employeeNumber",
			},
		},
	}
	for _, c := range cases {
		path, err := ParsePath(c.Input)
		require.NoError(t, err, c.Input)
		require.Equal(t, c.Want, path, c.Input)
	}
}

func TestExpression(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Input string
		Want  *Expression
	}{
		{
			`userName eq "bjensen"`,
			&Expression{
				Path: Path{
					AttrName: "userName",
				},
				CompareOp: "eq",
				Value:     `"bjensen"`,
			},
		},
		{
			`name.familyName co "O'Malley"`,
			&Expression{
				Path: Path{
					AttrName:    "name",
					SubAttrName: "familyName",
				},
				CompareOp: "co",
				Value:     `"O'Malley"`,
			},
		},
		{
			`urn:ietf:params:scim:schemas:core:2.0:User:userName sw "J"`,
			&Expression{
				Path: Path{
					URI:      "urn:ietf:params:scim:schemas:core:2.0:User",
					AttrName: "userName",
				},
				CompareOp: "sw",
				Value:     `"J"`,
			},
		},
		{
			`title pr`,
			&Expression{
				Path: Path{
					AttrName: "title",
				},
				CompareOp: "pr",
			},
		},
		{
			`emails[type eq "work" and value co "@example.com"]`,
			&Expression{
				Path: Path{
					AttrName: "emails",
					Filter: &OrExpression{
						Left: &AndTerm{
							Left: Expression{
								Path: Path{
									AttrName: "type",
								},
								CompareOp: "eq",
								Value:     `"work"`,
							},
							Right: []Expr{
								Expression{
									Path: Path{
										AttrName: "value",
									},
									CompareOp: "co",
									Value:     `"@example.com"`,
								},
							},
						},
					},
				},
			},
		},
		{
			`emails[type eq "work" and value co "@example.com" and value ne "@example2.com"]`,
			&Expression{
				Path: Path{
					AttrName: "emails",
					Filter: &OrExpression{
						Left: &AndTerm{
							Left: Expression{
								Path: Path{
									AttrName: "type",
								},
								CompareOp: "eq",
								Value:     `"work"`,
							},
							Right: []Expr{
								Expression{
									Path: Path{
										AttrName: "value",
									},
									CompareOp: "co",
									Value:     `"@example.com"`,
								},
								Expression{
									Path: Path{
										AttrName: "value",
									},
									CompareOp: "ne",
									Value:     `"@example2.com"`,
								},
							},
						},
					},
				},
			},
		},
		{
			`emails[type eq "work" or value co "@example.com" or value ne "@example2.com"]`,
			&Expression{
				Path: Path{
					AttrName: "emails",
					Filter: &OrExpression{
						Left: &AndTerm{
							Left: Expression{
								Path: Path{
									AttrName: "type",
								},
								CompareOp: "eq",
								Value:     `"work"`,
							},
						},
						Right: []*AndTerm{
							{
								Left: Expression{
									Path: Path{
										AttrName: "value",
									},
									CompareOp: "co",
									Value:     `"@example.com"`,
								},
							},
							{
								Left: Expression{
									Path: Path{
										AttrName: "value",
									},
									CompareOp: "ne",
									Value:     `"@example2.com"`,
								},
							},
						},
					},
				},
			},
		},
	}

	parser := participle.MustBuild[Expression](parserOptions()...)
	for _, c := range cases {
		path, err := parser.ParseString("", c.Input)
		require.NoError(t, err, c.Input)
		require.Equal(t, c.Want, path)
	}
}

func TestParser(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Input string
		Want  *OrExpression
	}{
		{
			`userName eq "bjensen"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "userName",
						},
						CompareOp: "eq",
						Value:     `"bjensen"`,
					},
				},
			},
		},
		{ // test conflict tokenize, 'pr'
			`preferredLanguage eq "en"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "preferredLanguage",
						},
						CompareOp: "eq",
						Value:     `"en"`,
					},
				},
			},
		},
		{
			`urn:ietf:params:scim:schemas:core:2.0:User:userName sw "J"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							URI:      "urn:ietf:params:scim:schemas:core:2.0:User",
							AttrName: "userName",
						},
						CompareOp: "sw",
						Value:     `"J"`,
					},
				},
			},
		},
		{
			`title pr and userType eq "Employee"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "title",
						},
						CompareOp: "pr",
					},
					Right: []Expr{
						Expression{
							Path: Path{
								AttrName: "userType",
							},
							CompareOp: "eq",
							Value:     `"Employee"`,
						},
					},
				},
			},
		},
		{
			`not (title pr and userType eq "Employee")`,
			&OrExpression{
				Left: &AndTerm{
					Left: NotExpression{
						Not: true,
						Group: OrExpression{
							Left: &AndTerm{
								Left: Expression{
									Path: Path{
										AttrName: "title",
									},
									CompareOp: "pr",
								},
								Right: []Expr{
									Expression{
										Path: Path{
											AttrName: "userType",
										},
										CompareOp: "eq",
										Value:     `"Employee"`,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			`title pr and not (userType eq "Employee")`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "title",
						},
						CompareOp: "pr",
					},
					Right: []Expr{
						NotExpression{
							Not: true,
							Group: OrExpression{
								Left: &AndTerm{
									Left: Expression{
										Path: Path{
											AttrName: "userType",
										},
										CompareOp: "eq",
										Value:     `"Employee"`,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			`schemas eq "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "schemas",
						},
						CompareOp: "eq",
						Value:     `"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"`,
					},
				},
			},
		},
		{
			`emails co "example.com" or emails.value co "example.org"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "emails",
						},
						CompareOp: "co",
						Value:     `"example.com"`,
					},
				},
				Right: []*AndTerm{
					{
						Left: Expression{
							Path: Path{
								AttrName:    "emails",
								SubAttrName: "value",
							},
							CompareOp: "co",
							Value:     `"example.org"`,
						},
					},
				},
			},
		},
		{
			`userType ne "Employee" and not (emails co "example.com" or emails.value co "example.org")`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "userType",
						},
						CompareOp: "ne",
						Value:     `"Employee"`,
					},
					Right: []Expr{
						NotExpression{
							Not: true,
							Group: OrExpression{
								Left: &AndTerm{
									Left: Expression{
										Path: Path{
											AttrName: "emails",
										},
										CompareOp: "co",
										Value:     `"example.com"`,
									},
								},
								Right: []*AndTerm{
									{
										Left: Expression{
											Path: Path{
												AttrName:    "emails",
												SubAttrName: "value",
											},
											CompareOp: "co",
											Value:     `"example.org"`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			`userType ne "Employee" and (emails co "example.com" or emails.value co "example.org")`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "userType",
						},
						CompareOp: "ne",
						Value:     `"Employee"`,
					},
					Right: []Expr{
						NotExpression{
							Group: OrExpression{
								Left: &AndTerm{
									Left: Expression{
										Path: Path{
											AttrName: "emails",
										},
										CompareOp: "co",
										Value:     `"example.com"`,
									},
								},
								Right: []*AndTerm{
									{
										Left: Expression{
											Path: Path{
												AttrName:    "emails",
												SubAttrName: "value",
											},
											CompareOp: "co",
											Value:     `"example.org"`,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			`(userType ne "Employee" or emails co "example.com") and emails.value co "example.org"`,
			&OrExpression{
				Left: &AndTerm{
					Left: NotExpression{
						Group: OrExpression{
							Left: &AndTerm{
								Left: Expression{
									Path: Path{
										AttrName: "userType",
									},
									CompareOp: "ne",
									Value:     `"Employee"`,
								},
							},
							Right: []*AndTerm{
								{
									Left: Expression{
										Path: Path{
											AttrName: "emails",
										},
										CompareOp: "co",
										Value:     `"example.com"`,
									},
								},
							},
						},
					},
					Right: []Expr{
						Expression{
							Path: Path{
								AttrName:    "emails",
								SubAttrName: "value",
							},
							CompareOp: "co",
							Value:     `"example.org"`,
						},
					},
				},
			},
		},
		{
			`emails[type eq "work" and value co "@example.com"] or ims[type eq "xmpp" and value co "@foo.com"]`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "emails",
							Filter: &OrExpression{
								Left: &AndTerm{
									Left: Expression{
										Path: Path{
											AttrName: "type",
										},
										CompareOp: "eq",
										Value:     `"work"`,
									},
									Right: []Expr{
										Expression{
											Path: Path{
												AttrName: "value",
											},
											CompareOp: "co",
											Value:     `"@example.com"`,
										},
									},
								},
							},
						},
					},
				},
				Right: []*AndTerm{
					{
						Left: Expression{
							Path: Path{
								AttrName: "ims",
								Filter: &OrExpression{
									Left: &AndTerm{
										Left: Expression{
											Path: Path{
												AttrName: "type",
											},
											CompareOp: "eq",
											Value:     `"xmpp"`,
										},
										Right: []Expr{
											Expression{
												Path: Path{
													AttrName: "value",
												},
												CompareOp: "co",
												Value:     `"@foo.com"`,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			// Logical operators - where "not" takes precedence over "and", which takes precedence over "or"
			// https://datatracker.ietf.org/doc/html/rfc7644#section-3.4.2.2
			`userType eq "A" or userType eq "B" and userType ne "C" or userType eq "D"`,
			&OrExpression{
				Left: &AndTerm{
					Left: Expression{
						Path: Path{
							AttrName: "userType",
						},
						CompareOp: "eq",
						Value:     `"A"`,
					},
				},
				Right: []*AndTerm{
					{
						Left: Expression{
							Path: Path{
								AttrName: "userType",
							},
							CompareOp: "eq",
							Value:     `"B"`,
						},
						Right: []Expr{
							Expression{
								Path: Path{
									AttrName: "userType",
								},
								CompareOp: "ne",
								Value:     `"C"`,
							},
						},
					},
					{
						Left: Expression{
							Path: Path{
								AttrName: "userType",
							},
							CompareOp: "eq",
							Value:     `"D"`,
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		path, err := ParseFilter(c.Input)
		require.NoError(t, err, c.Input)
		require.Equal(t, c.Want, path, c.Input)
		require.Equal(t, c.Input, path.String())
	}
}

func TestParseFilterOperatorCaseInsensitive(t *testing.T) {
	t.Parallel()
	for _, op := range []string{"eq", "Eq", "EQ"} {
		parsed, err := ParseFilter(fmt.Sprintf(`userName %s "bjensen"`, op))
		require.NoError(t, err, op)
		expr, ok := parsed.Left.Left.(Expression)
		require.True(t, ok, op)
		require.Equal(t, OpEQ, expr.CompareOp, op)
	}

	parsed, err := ParseFilter(`userName eq "a" OR userName eq "b"`)
	require.NoError(t, err)
	require.Len(t, parsed.Right, 1)

	path, err := ParsePath(`emails[type Eq "work"].value`)
	require.NoError(t, err)
	require.Equal(t, "emails", path.AttrName)
	require.NotNil(t, path.Filter)
}

// A PATH is attrPath / valuePath [subAttr], so a bare attrPath is valid even
// though it is not a valid FILTER. Only the bracketed valFilter is a FILTER.
func TestParsePathBareAttrPath(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"userName", "name.familyName", "emails", `emails[type eq "work"].value`} {
		_, err := ParsePath(input)
		require.NoError(t, err, input)
	}
	_, err := ParsePath(`emails[userName]`)
	require.ErrorContains(t, err, "requires a compare operator")

	_, err = ParseFilter(`emails[type eq "work"]`)
	require.NoError(t, err)
	_, err = ParseFilter(`emails[type eq "work"].value eq "x"`)
	require.NoError(t, err)
}
