package graph

import (
	"reflect"

	"github.com/graphql-go/graphql"
)

// Introspectable is implemented by every QueryField, MutationField, and
// SubscriptionField this library produces. External tooling (dashboards,
// documentation generators, schema linters) can iterate a schema's fields
// and read their metadata without parsing Go source.
//
// Because FieldInfo is a plain struct, the output is safe to JSON-serialize,
// pass across goroutines, and diff between builds.
//
// Typical consumer:
//
//	for _, qf := range schemaParams.QueryFields {
//	    info, ok := graph.Inspect(qf)
//	    if !ok { continue }
//	    fmt.Printf("%s(%v) -> %s [mws=%d]\n",
//	        info.Name, info.Args, info.ReturnType, len(info.Middlewares))
//	}
type Introspectable interface {
	FieldInfo() FieldInfo
}

// Inspect returns the FieldInfo for any value implementing Introspectable,
// or a zero FieldInfo and false otherwise. Use this when you hold a generic
// QueryField / MutationField interface reference rather than the concrete
// *UnifiedResolver — it saves the type assertion boilerplate.
func Inspect(f any) (FieldInfo, bool) {
	if ii, ok := f.(Introspectable); ok {
		return ii.FieldInfo(), true
	}
	return FieldInfo{}, false
}

// FieldKind distinguishes query / mutation / subscription at the metadata level.
type FieldKind string

const (
	FieldKindQuery        FieldKind = "query"
	FieldKindMutation     FieldKind = "mutation"
	FieldKindSubscription FieldKind = "subscription"
)

// FieldInfo is a serializable snapshot of a resolver's declared metadata.
// It captures everything a tool would need to render documentation or a
// dashboard view of the field — without calling Serve() or parsing SDL.
type FieldInfo struct {
	Name              string           `json:"name"`
	Description       string           `json:"description,omitempty"`
	Kind              FieldKind        `json:"kind"`
	ReturnType        string           `json:"returnType"` // SDL-style, e.g. "User!" or "[Advert]"
	List              bool             `json:"list,omitempty"`
	Paginated         bool             `json:"paginated,omitempty"`
	Deprecated        bool             `json:"deprecated,omitempty"`
	DeprecationReason string           `json:"deprecationReason,omitempty"`
	Args              []ArgInfo        `json:"args,omitempty"`
	Middlewares       []MiddlewareInfo `json:"middlewares,omitempty"`
	InputObject       *InputObjectInfo `json:"inputObject,omitempty"`
}

// ArgInfo describes one argument on a resolver. Required is true iff the
// underlying GraphQL type is NonNull. DefaultValue reflects whatever was
// supplied to WithArgDefault or struct-tag defaults; it's nil otherwise.
type ArgInfo struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"` // SDL-style
	Description  string          `json:"description,omitempty"`
	Required     bool            `json:"required,omitempty"`
	DefaultValue any             `json:"defaultValue,omitempty"`
	Validators   []ValidatorInfo `json:"validators,omitempty"`
}

// MiddlewareInfo is the metadata recorded by WithNamedMiddleware. Middlewares
// added via plain WithMiddleware have Name == "anonymous".
type MiddlewareInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// InputObjectInfo describes the input struct for mutations built with
// WithInputObject. TypeName is the Go type name; the dashboard can fetch
// field-level detail via standard graphql-go introspection.
type InputObjectInfo struct {
	TypeName string `json:"typeName"`
	Nullable bool   `json:"nullable,omitempty"`
}

// ValidatorInfo describes a per-argument validator for dashboard display.
// The Kind field identifies the validator family (so a UI can render a
// matching icon or tooltip); Details holds the parameters (e.g.
// {"min":1,"max":100} for StringLength).
type ValidatorInfo struct {
	Kind    string `json:"kind"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

// ArgValidator is the runtime shape of a validator. Return nil when the
// value is acceptable; return an error whose message is user-facing.
type ArgValidator func(value any) error

// Validator pairs a runtime function with its metadata. Use the helpers in
// validators.go (Required, StringLength, StringMatch, ...) to construct
// these rather than populating by hand.
type Validator struct {
	Info ValidatorInfo
	Fn   ArgValidator
}

// typeString renders any graphql.Type as its SDL-style string ("String!",
// "[Int]", "User"). Used by FieldInfo and by external tooling via GetArgInfo.
func typeString(t graphql.Type) string {
	if t == nil {
		return ""
	}
	return t.String()
}

// isNonNull reports whether an Input is wrapped in NonNull.
func isNonNull(t graphql.Input) bool {
	_, ok := t.(*graphql.NonNull)
	return ok
}

// argsToInfo converts a graphql.FieldConfigArgument into a sorted []ArgInfo,
// attaching any validator metadata from validators.
func argsToInfo(args graphql.FieldConfigArgument, validators map[string][]Validator) []ArgInfo {
	if len(args) == 0 {
		return nil
	}
	out := make([]ArgInfo, 0, len(args))
	for name, cfg := range args {
		vs := validators[name]
		vinfo := make([]ValidatorInfo, 0, len(vs))
		for _, v := range vs {
			vinfo = append(vinfo, v.Info)
		}
		out = append(out, ArgInfo{
			Name:         name,
			Type:         typeString(cfg.Type),
			Description:  cfg.Description,
			Required:     isNonNull(cfg.Type),
			DefaultValue: cfg.DefaultValue,
			Validators:   vinfo,
		})
	}
	// Deterministic order for diff-friendly snapshots.
	sortArgInfos(out)
	return out
}

func sortArgInfos(a []ArgInfo) {
	// Small slices — insertion sort keeps this dependency-free.
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1].Name > a[j].Name; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

// inputObjectInfo extracts InputObjectInfo for resolvers that set
// useInputObject. Returns nil otherwise.
func inputObjectInfo(inputType any, nullable bool, use bool) *InputObjectInfo {
	if !use || inputType == nil {
		return nil
	}
	t := reflect.TypeOf(inputType)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil {
		return nil
	}
	return &InputObjectInfo{TypeName: t.Name(), Nullable: nullable}
}