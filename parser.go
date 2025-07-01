package scimprotocol

import (
	"fmt"
	"reflect"
	"regexp"

	sq "github.com/Masterminds/squirrel"
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/memsql/errors"

	"singlestore.com/helios/scim/scimprotocol/scimerror"
)

// Filter Parse doc: https://datatracker.ietf.org/doc/html/rfc7644#section-3.4.2.2

const (
	uriPattrn     = `urn:ietf:params:scim:schemas:(core|extension:[^:]+):2\.0(:[^:]+)`
	numberPattern = `-?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`
	stringPattern = `"(?:[^\\"]|\\.)*"`
	// stringPattern = `"[^\\"]*"`
)

var URIPattenRegexp = regexp.MustCompile(uriPattrn)

// NOTE:
//   - Should avoid conflict tokenizer, and order matters.
var Lex = lexer.MustSimple([]lexer.SimpleRule{
	// compValue = false / null / true / number / string; rules from JSON (RFC 7159)
	{Name: "CompValue", Pattern: fmt.Sprintf(`(?:false|null|true|%s|%s)`, numberPattern, stringPattern)},
	{Name: "(", Pattern: `\(`},
	{Name: ")", Pattern: `\)`},
	{Name: "[", Pattern: `\[`},
	{Name: "]", Pattern: `\]`},
	{Name: "Not", Pattern: `not[ \t]+`},
	{Name: "URI", Pattern: uriPattrn},
	{Name: "AttrName", Pattern: `[a-zA-Z][-a-zA-Z0-9_]*`},
	{Name: "Whitespace", Pattern: `[ \t]+`},
	{Name: "Dot", Pattern: `\.`},
	{Name: "Colon", Pattern: `\:`},
})

// SQLGenerator is a interface used to convert expression to sql with SICM tag
type SQLGenerator interface {
	// sq.And, sq.Or, sq.Eq .... returns sq.Sqlizer
	// sqsq put error in sq.Sqlizer
	Generate(Common, bool) (sq.Sqlizer, error)
}

// Three cases about group of simple expression: not (group) |(group)| group.
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
	Common
}

type Common interface {
	Eval(v reflect.Value, azureAdd bool) (bool, error)
	String() string
	// sqsq will save joins func in SQLGenerator, not nice?
	// sqsq put error in sq.Sqlizer
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

type Expression struct { // compare expression
	Path Path `parser:"@@"`
	// check CompareOp in grammar to avoid tokenize conflict
	CompareOp string `parser:"(Whitespace (@'pr' | (@('eq'|'ne'|'co'|'sw'|'ew'|'gt'|'lt'|'ge'|'le')"`
	Value     string `parser:"  Whitespace @CompValue)))?"`
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
		return schemaFilterHelper(coreSchema, extensions, e.Value)
	}

	// will filling default schema, MUST have schema for the later filter function
	p := e.Path.GetNodes(coreSchema)
	return EvalHelper(e, p, v, nil, azureAdd)
}

func (e Expression) ToSqlizer(sg SQLGenerator, not bool) (sq.Sqlizer, error) {
	return sg.Generate(e, not)
}

type Path struct {
	URI         string        `parser:"(@URI"`
	Colon       string        `parser:"  ':')?"`
	AttrName    string        `parser:"@AttrName"`
	Filter      *OrExpression `parser:"('[' Whitespace? @@  Whitespace?']')?"`
	SubAttrName string        `parser:"('.'@AttrName)?"`
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

// 'not' has highest precedence so put it in deepest level
type NotExpression struct {
	Not   bool         `parser:"(@Not "`
	Group OrExpression `parser:"  '('@@')') | ('('@@')')"`
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

func ParseFilter(filterStr string) (*OrExpression, error) {
	if len(filterStr) == 0 {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Errorf("input filter is empty"))
	}
	filterParser := participle.MustBuild[OrExpression](
		participle.Lexer(Lex),
		// order is matters
		participle.Union[Expr](Expression{}, NotExpression{}),
	)
	filter, err := filterParser.ParseString("", filterStr)
	if err != nil {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "failed to parse filter, %s", filterStr))
	}
	return filter, nil
}

func ParsePath(pathStr string) (*Path, error) {
	if len(pathStr) == 0 {
		return nil, nil // path could be empty when doing patch
	}
	filterParser := participle.MustBuild[Path](
		participle.Lexer(Lex),
		participle.Union[Expr](Expression{}, NotExpression{}),
	)
	path, err := filterParser.ParseString("", pathStr)
	if err != nil {
		return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidFilter, errors.Wrapf(err, "failed to parse path, %s", pathStr))
	}
	return path, nil
}
