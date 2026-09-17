package scimprotocol

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/memsql/errors"
	"github.com/singlestore-labs/scim/scimerror"
)

// Filter Parse doc: https://datatracker.ietf.org/doc/html/rfc7644#section-3.4.2.2

const (
	uriPattrn     = `urn:ietf:params:scim:schemas:(core|extension:[^:]+):2\.0(:[^:]+)`
	numberPattern = `-?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`
	stringPattern = `"(?:[^\\"]|\\.)*"`
	// stringPattern = `"[^\\"]*"`
)

var URIPattenRegexp = regexp.MustCompile(uriPattrn)

// Lex tokenizes shapes only, never keywords.
//
// Rules are distinguishable by their first character, so ordering is only
// significant for URI, which must precede Ident.
var Lex = lexer.MustSimple([]lexer.SimpleRule{
	// compValue = false / null / true / number / string; rules from JSON (RFC 7159).
	// 'false', 'null' and 'true' are Ident-shaped and handled in the grammar.
	{Name: "String", Pattern: stringPattern},
	{Name: "Number", Pattern: numberPattern},
	{Name: "URI", Pattern: uriPattrn},
	{Name: "Ident", Pattern: `[a-zA-Z][-a-zA-Z0-9_]*`}, // attribute name, compare operator, logical operator
	{Name: "(", Pattern: `\(`},
	{Name: ")", Pattern: `\)`},
	{Name: "[", Pattern: `\[`},
	{Name: "]", Pattern: `\]`},
	{Name: "Whitespace", Pattern: `[ \t]+`},
	{Name: "Dot", Pattern: `\.`},
	{Name: "Colon", Pattern: `\:`},
})

type CompareOp string

// RFC 7644 §3.4.2.2 attribute operators. The parser tag on Expression is
// the allow-list; these constants are for Go comparisons after Capture.
const (
	OpPR CompareOp = "pr"
	OpEQ CompareOp = "eq"
	OpNE CompareOp = "ne"
	OpCO CompareOp = "co"
	OpSW CompareOp = "sw"
	OpEW CompareOp = "ew"
	OpGT CompareOp = "gt"
	OpLT CompareOp = "lt"
	OpGE CompareOp = "ge"
	OpLE CompareOp = "le"
)

var _ participle.Capture = (*CompareOp)(nil)

// Capture normalizes the operator, which RFC 7644 §3.4.2.2 defines as
// case-insensitive, so callers can compare against the constants above.
func (c *CompareOp) Capture(values []string) error {
	*c = CompareOp(strings.ToLower(values[0]))
	return nil
}

// CompValue holds the raw JSON literal of a comparison value, quotes included
// for strings, so that it stays consumable by json.Unmarshal.
//
//	compValue = false / null / true / number / string ; rules from JSON (RFC 7159)
type CompValue string

var _ participle.Capture = (*CompValue)(nil)

// Capture stores the raw token. JSON keywords must already be lowercase
// (RFC 7159). Ident matching is case-insensitive for names and operators, so
// TRUE would otherwise parse as the keyword 'true'.
func (c *CompValue) Capture(values []string) error {
	value := values[0]
	if lower := strings.ToLower(value); lower == "true" || lower == "false" || lower == "null" {
		if value != lower {
			return errors.Errorf("JSON keyword %q must be lowercase", value)
		}
	}
	*c = CompValue(value)
	return nil
}

// SQLGenerator is a interface used to convert expression to sql with SICM tag
type SQLGenerator interface {
	// sq.And, sq.Or, sq.Eq .... returns sq.Sqlizer
	Generate(Common, bool) (sq.Sqlizer, error)
}

// Three cases about group of simple expression: not (group) | (group) | group.
//
// From RFC: https://datatracker.ietf.org/doc/html/rfc7644#section-3.4.2.2
// Filters MUST be evaluated using the following order of operations, in
// order of precedence:
//  1. Grouping operators
//  2. Logical operators - where "not" takes precedence over "and",
//     which takes precedence over "or"
//  3. Attribute operators

// Expr should be implement by Expression and NotExpression (which include OrExpression)
type Expr interface {
	expr()
	RedactedString() string
	// validate enforces the RFC 7644 filter rules that the grammar cannot
	// express. See [Expression.validate].
	validate() error
	Common
}

type Common interface {
	Eval(v reflect.Value, azureAdd bool) (bool, error)
	String() string
	ToSqlizer(sg SQLGenerator, not bool) (sq.Sqlizer, error)
}

type OrExpression struct {
	Left  *AndTerm   `parser:"@@"`
	Right []*AndTerm `parser:"(Whitespace 'or' Whitespace @@)*"`
}

func (oe OrExpression) String() string {
	result := oe.Left.String()
	for _, r := range oe.Right {
		result = fmt.Sprintf("%s or %s", result, r.String())
	}
	return result
}

func (oe OrExpression) ToSqlizer(sg SQLGenerator, not bool) (sq.Sqlizer, error) {
	return sg.Generate(oe, not)
}

func (oe OrExpression) RedactedString() string {
	result := oe.Left.RedactedString()
	for _, r := range oe.Right {
		result = fmt.Sprintf("%s or %s", result, r.String())
	}
	return result
}

// Eval evaluates the OrExpression against the given reflect.Value v.
// The azureAdd parameter indicates whether to apply Azure-specific filtering logic. Detail explained in filter.go [EvalHelper].
func (le OrExpression) Eval(v reflect.Value, azureAdd bool) (bool, error) {
	value, err := le.Left.Eval(v, azureAdd)
	if err != nil {
		return false, err
	}
	for _, expr := range le.Right {
		rightValue, err := expr.Eval(v, azureAdd)
		if err != nil {
			return false, err
		}
		value = value || rightValue
	}
	return value, nil
}

func (oe *OrExpression) validate() error {
	if oe == nil {
		return nil
	}
	if err := oe.Left.validate(); err != nil {
		return err
	}
	for _, term := range oe.Right {
		if err := term.validate(); err != nil {
			return err
		}
	}
	return nil
}

type AndTerm struct {
	Left  Expr   `parser:"@@"` // Expr will be recursive parse either Expression or Not(Group)
	Right []Expr `parser:"(Whitespace 'and' Whitespace @@)*"`
}

func (e AndTerm) String() string {
	result := e.Left.String()
	for _, r := range e.Right {
		result = fmt.Sprintf("%s and %s", result, r.String())
	}
	return result
}

func (e AndTerm) ToSqlizer(sg SQLGenerator, not bool) (sq.Sqlizer, error) {
	return sg.Generate(e, not)
}

func (e AndTerm) RedactedString() string {
	result := e.Left.RedactedString()
	for _, r := range e.Right {
		result = fmt.Sprintf("%s and %s", result, r.String())
	}
	return result
}

func (ot AndTerm) Eval(v reflect.Value, azureAdd bool) (bool, error) {
	value, err := ot.Left.Eval(v, azureAdd)
	if err != nil {
		return false, err
	}
	for _, expr := range ot.Right {
		rightValue, err := expr.Eval(v, azureAdd)
		if err != nil {
			return false, err
		}
		value = value && rightValue
	}
	return value, nil
}

func (at *AndTerm) validate() error {
	if at == nil {
		return nil
	}
	if at.Left != nil {
		if err := at.Left.validate(); err != nil {
			return err
		}
	}
	for _, expr := range at.Right {
		if err := expr.validate(); err != nil {
			return err
		}
	}
	return nil
}

// Expression is an attribute expression. 'pr' takes no value while every other
// operator requires one, so the two forms are separate alternatives:
//
//	attrExp = (attrPath SP "pr") / (attrPath SP compareOp SP compValue)
type Expression struct {
	Path Path `parser:"@@"`
	// Operators and the JSON keywords are Ident tokens, so they are matched as
	// literals by position here instead of being reserved by the lexer.
	//
	// Operator is optional so a valuePath FILTER can be Path "[" valFilter "]"
	// with no compareOp after ']'.
	CompareOp CompareOp `parser:"(Whitespace (@'pr' | (@('eq'|'ne'|'co'|'sw'|'ew'|'gt'|'lt'|'ge'|'le')"`
	Value     CompValue `parser:"  Whitespace @(String|Number|'true'|'false'|'null'))))?"`
}

var _ Expr = (*Expression)(nil)

func (Expression) expr() {}

func (e Expression) String() string {
	result := e.Path.String()
	if e.CompareOp != "" {
		result = fmt.Sprintf("%s %s", result, e.CompareOp)
	}
	if e.Value != "" {
		result = fmt.Sprintf("%s %s", result, e.Value)
	}
	return result
}

func (e Expression) RedactedString() string {
	result := e.Path.String()
	if e.CompareOp != "" {
		result = fmt.Sprintf("%s %s", result, e.CompareOp)
	}
	if e.Value != "" {
		result = fmt.Sprintf("%s %s", result, "x")
	}
	return result
}

func (e Expression) Eval(v reflect.Value, azureAdd bool) (bool, error) {
	t := v.Type()
	if t.Kind() != reflect.Struct {
		return false, errors.Errorf("evaluate expression need a struct but %s (type:%s)", t.Kind(), v.Type())
	}

	coreSchema, extensions, err := GetSchemaURIFromResource(t, nil)
	if err != nil && !errors.Is(err, scimerror.ErrNotFound) { // tolerate non-schema, may filtering on multi-value attribute
		return false, err
	}

	// special handle for `schemas eq "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"`
	if e.Path.AttrName == "schemas" {
		return schemaFilterHelper(coreSchema, extensions, string(e.Value))
	}

	// will filling default schema, MUST have schema for the later filter function
	p := e.Path.GetNodes(coreSchema)
	return EvalHelper(e, p, v, nil, azureAdd)
}

func (e Expression) ToSqlizer(sg SQLGenerator, not bool) (sq.Sqlizer, error) {
	return sg.Generate(e, not)
}

// validate rejects a bare attrPath used as a FILTER. RFC 7644 §3.4.2.2 allows
// attrExp (operator required) or valuePath (attrPath "[" valFilter "]"), not an
// attribute name alone. The grammar keeps the operator optional so valuePath
// can omit it; this is the rest of that distinction.
func (e Expression) validate() error {
	if e.CompareOp == "" && e.Path.Filter == nil {
		return errors.Errorf("filter on attribute %q requires a compare operator or a value path [...]", e.Path.AttrName)
	}
	return e.Path.Filter.validate()
}

type Path struct {
	URI         string        `parser:"(@URI"`
	Colon       string        `parser:"  ':')?"`
	AttrName    string        `parser:"@Ident"`
	Filter      *OrExpression `parser:"('[' Whitespace? @@  Whitespace?']')?"`
	SubAttrName string        `parser:"('.'@Ident)?"`
}

func (p Path) String() string {
	result := p.AttrName
	if p.URI != "" {
		result = fmt.Sprintf("%s:%s", p.URI, result)
	}
	if p.Filter != nil {
		result = fmt.Sprintf("%s[%s]", result, p.Filter.String())
	}
	if p.SubAttrName != "" {
		result = fmt.Sprintf("%s.%s", result, p.SubAttrName)
	}
	return result
}

type Node struct {
	Value any
	Next  *Node
}

// GetNodes returns a linked list from Path for recursive when filter, patch and marshal.
// It returns dummyhead node (empty node) followed by path nodes.
// If path is nil (no path), then return nil.
// If the node is the last one (end path), then node.Next is nil.
func (p *Path) GetNodes(coreSchemaID string) *Node {
	dummyHead := &Node{}
	if p == nil {
		return dummyHead
	}
	cur := dummyHead
	// set default uri
	if p.URI == "" {
		p.URI = coreSchemaID
	}
	if p.URI != "" {
		// If path field is not empty then add to tail of the linked list
		cur.Next = &Node{}
		cur = cur.Next
		cur.Value = p.URI
	}
	if p.AttrName != "" {
		cur.Next = &Node{}
		cur = cur.Next
		cur.Value = p.AttrName
	}
	if p.Filter != nil {
		cur.Next = &Node{}
		cur = cur.Next
		cur.Value = *(p.Filter)
	}
	if p.SubAttrName != "" {
		cur.Next = &Node{}
		cur = cur.Next
		cur.Value = p.SubAttrName
	}
	return dummyHead
}

// NotExpression is a grouped filter, optionally negated. 'not' has the highest
// precedence so it sits at the deepest level.
//
//	FILTER = ... / *1"not" "(" FILTER ")"
//
// The ABNF puts no SP after "not" while the RFC examples and identity providers
// send "not (", so both are accepted. 'not' is an Ident token rather than a
// reserved word, which keeps an attribute named "not" parseable.
type NotExpression struct {
	Not   bool         `parser:"(@'not' Whitespace?)?"`
	Group OrExpression `parser:"'(' Whitespace? @@ Whitespace? ')'"`
}

var _ Expr = (*NotExpression)(nil)

func (NotExpression) expr() {}

func (sg NotExpression) Eval(v reflect.Value, azureAdd bool) (bool, error) {
	pass, err := sg.Group.Eval(v, azureAdd)
	if err != nil {
		return false, err
	}
	if sg.Not {
		return !pass, err
	}
	return pass, err
}

func (ne NotExpression) String() string {
	if ne.Not {
		return fmt.Sprintf("not (%s)", ne.Group.String())
	} else {
		return fmt.Sprintf("(%s)", ne.Group.String())
	}
}

func (ne NotExpression) ToSqlizer(sg SQLGenerator, not bool) (sq.Sqlizer, error) {
	return sg.Generate(ne, not)
}

func (ne NotExpression) RedactedString() string {
	if ne.Not {
		return fmt.Sprintf("not (%s)", ne.Group.RedactedString())
	} else {
		return fmt.Sprintf("(%s)", ne.Group.RedactedString())
	}
}

func (ne NotExpression) validate() error {
	return ne.Group.validate()
}

func parserOptions() []participle.Option {
	return []participle.Option{
		participle.Lexer(Lex),
		// order matters: 'not' is Ident-shaped, so try the grouping form before
		// falling back to reading it as an attribute name.
		participle.Union[Expr](NotExpression{}, Expression{}),
		// RFC 7644 §3.4.2.2: attribute names and operators are case-insensitive.
		participle.CaseInsensitive("Ident"),
		// Keywords double as attribute names, so alternatives must backtrack.
		participle.UseLookahead(3),
	}
}

func ParseFilter(filterStr string) (*OrExpression, error) {
	if len(filterStr) == 0 {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("input filter is empty"))
	}
	filterParser := participle.MustBuild[OrExpression](parserOptions()...)
	filter, err := filterParser.ParseString("", filterStr)
	if err != nil {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "failed to parse filter, %s", filterStr))
	}
	if err := filter.validate(); err != nil {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "failed to parse filter, %s", filterStr))
	}
	return filter, nil
}

func ParsePath(pathStr string) (*Path, error) {
	if len(pathStr) == 0 {
		return nil, nil // path could be empty when doing patch
	}
	filterParser := participle.MustBuild[Path](parserOptions()...)
	path, err := filterParser.ParseString("", pathStr)
	if err != nil {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "failed to parse path, %s", pathStr))
	}
	// only the bracketed valFilter is a FILTER, a bare attrPath is a valid PATH
	if err := path.Filter.validate(); err != nil {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "failed to parse path, %s", pathStr))
	}
	return path, nil
}
