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
	{
		t.Log("test CompValue regex pattern")
		rules := Lex.Rules()["Root"]
		curRule := rules[0]
		require.Equal(t, "CompValue", curRule.Name, "this test should test rule 'CompValue', if not, please change index")
		t.Logf("pattern: %s", curRule.Pattern)
		re, err := regexp.Compile(curRule.Pattern)
		require.NoError(t, err, "invalid tokenize regex on %s, pattern: %s", curRule.Name, curRule.Pattern)
		require.False(t, re.MatchString(`test`), `testing [test] failed`)
		require.True(t, re.MatchString(`"test"`), `testing ["test"] failed`)
		require.True(t, re.MatchString(`false`), `testing [false] failed`)
		require.True(t, re.MatchString(`true`), `testing [true] failed`)
		require.True(t, re.MatchString(`"123"`), `testing ["123"] failed`)
		require.True(t, re.MatchString(`123`), `testing [true] failed`)
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
	parser := participle.MustBuild[Expression](
		participle.Lexer(Lex),
		participle.Union[Expr](Expression{}, NotExpression{}),
	)
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
