package graph

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/graphql-go/graphql"
)

// NewMutation is the single entry point for building mutations. The returned
// MutationBuilder cannot produce a GraphQL field on its own — you must pick a
// kind (Create, Update, Delete, Action, or Upsert) before calling WithResolver
// and Build. The kind determines the resolver signature.
//
// Example:
//
//	graph.NewMutation[User, CreateUserInput]("createUser").
//	    Create().
//	    WithResolver(func(ctx context.Context, in CreateUserInput) (*User, error) {
//	        return userService.Create(ctx, in)
//	    }).
//	    Build()
func NewMutation[T any, In any](name string) *MutationBuilder[T, In] {
	return &MutationBuilder[T, In]{
		name:      name,
		inputName: "input",
	}
}

// MutationBuilder carries config common to every mutation kind. It deliberately
// exposes no WithResolver or Build — you must call Create/Update/Delete/Action/
// Upsert to transition to a kind-specific builder.
type MutationBuilder[T any, In any] struct {
	name        string
	description string
	inputName   string
	middlewares []FieldMiddleware
}

func (b *MutationBuilder[T, In]) WithDescription(d string) *MutationBuilder[T, In] {
	b.description = d
	return b
}

func (b *MutationBuilder[T, In]) WithInputName(n string) *MutationBuilder[T, In] {
	b.inputName = n
	return b
}

func (b *MutationBuilder[T, In]) Use(mw ...FieldMiddleware) *MutationBuilder[T, In] {
	b.middlewares = append(b.middlewares, mw...)
	return b
}

func (b *MutationBuilder[T, In]) Create() *CreateBuilder[T, In] {
	return &CreateBuilder[T, In]{base: b}
}

func (b *MutationBuilder[T, In]) Update() *UpdateBuilder[T, In] {
	return &UpdateBuilder[T, In]{base: b}
}

func (b *MutationBuilder[T, In]) Delete() *DeleteBuilder[T, In] {
	return &DeleteBuilder[T, In]{base: b}
}

func (b *MutationBuilder[T, In]) Action() *ActionBuilder[T, In] {
	return &ActionBuilder[T, In]{base: b}
}

func (b *MutationBuilder[T, In]) Upsert() *UpsertBuilder[T, In] {
	return &UpsertBuilder[T, In]{base: b}
}

// ---------- Error model ----------

type ErrorCode string

const (
	CodeInvalidInput ErrorCode = "INVALID_INPUT"
	CodeUnauthorized ErrorCode = "UNAUTHORIZED"
	CodeNotFound     ErrorCode = "NOT_FOUND"
	CodeConflict     ErrorCode = "CONFLICT"
	CodeInternal     ErrorCode = "INTERNAL"
)

type MutationError struct {
	Code    ErrorCode
	Field   string
	Message string
	Cause   error
}

func (e *MutationError) Error() string {
	msg := e.Message
	if msg == "" && e.Cause != nil {
		msg = e.Cause.Error()
	}
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, msg, e.Field)
	}
	return fmt.Sprintf("%s: %s", e.Code, msg)
}

func (e *MutationError) Unwrap() error { return e.Cause }

func (e *MutationError) Extensions() map[string]interface{} {
	ext := map[string]interface{}{"code": string(e.Code)}
	if e.Field != "" {
		ext["field"] = e.Field
	}
	return ext
}

func wrapAs(code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	var me *MutationError
	if errors.As(err, &me) {
		return me
	}
	return &MutationError{Code: code, Message: err.Error(), Cause: err}
}

// ---------- Lifecycle interfaces ----------

// InputNormalizer lets an input struct normalize its fields (trim spaces,
// lowercase emails, etc.) after decoding and before validation.
type InputNormalizer interface{ Normalize() }

// InputValidator runs after Normalize. A non-nil error short-circuits the
// resolver and is surfaced as INVALID_INPUT unless the returned error is
// already a *MutationError (in which case its Code is preserved).
type InputValidator interface {
	Validate(ctx context.Context) error
}

// InputAuthorizer runs before Validate. Failures surface as UNAUTHORIZED.
type InputAuthorizer interface {
	Authorize(ctx context.Context) error
}

// PatchInputValidator is preferred over InputValidator for Update and Upsert
// kinds, because it receives the set of fields the client actually sent.
type PatchInputValidator interface {
	ValidatePatch(ctx context.Context, present PresenceSet) error
}

// ---------- Presence set ----------

// PresenceSet reports which fields were included in the client request. It is
// populated from the raw args map, not from the decoded struct, so "field sent
// with zero value" and "field omitted" are distinguishable.
type PresenceSet interface {
	Has(field string) bool
	Fields() []string
}

type presenceSet map[string]struct{}

func (p presenceSet) Has(field string) bool {
	_, ok := p[field]
	return ok
}

func (p presenceSet) Fields() []string {
	out := make([]string, 0, len(p))
	for k := range p {
		out = append(out, k)
	}
	return out
}

// ---------- Patch[T] ----------

// Patch wraps a decoded input with the set of fields the client sent. Only
// Update and Upsert resolvers receive a Patch — the framework constructs it
// from the raw args, so "omitted" and "set to zero value" are distinguishable.
type Patch[T any] struct {
	data    T
	present presenceSet
}

func (p Patch[T]) Get() T { return p.data }

func (p Patch[T]) Has(field string) bool { return p.present.Has(field) }

func (p Patch[T]) Fields() []string { return p.present.Fields() }

func (p Patch[T]) Presence() PresenceSet { return p.present }

// Apply copies only the fields that were present in the request onto dst.
func (p Patch[T]) Apply(dst *T) {
	srcV := reflect.ValueOf(p.data)
	t := srcV.Type()
	if t.Kind() != reflect.Struct {
		return
	}
	dstV := reflect.ValueOf(dst).Elem()
	for _, f := range patchFieldsFor(t) {
		if _, ok := p.present[f.name]; ok {
			dstV.Field(f.index).Set(srcV.Field(f.index))
		}
	}
}

type patchField struct {
	name  string
	index int
}

var patchFieldCache sync.Map // map[reflect.Type][]patchField

// patchFieldsFor returns the cached (name, index) pairs for exported fields of t.
// Tag parsing happens once per type, not per Apply call.
func patchFieldsFor(t reflect.Type) []patchField {
	if v, ok := patchFieldCache.Load(t); ok {
		return v.([]patchField)
	}
	n := t.NumField()
	fields := make([]patchField, 0, n)
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		fields = append(fields, patchField{name: getFieldName(f), index: i})
	}
	actual, _ := patchFieldCache.LoadOrStore(t, fields)
	return actual.([]patchField)
}

// ---------- Result[T] ----------

// Result is the return value for an Upsert resolver. Created distinguishes
// insert from update so the generated payload type can expose it to clients.
type Result[T any] struct {
	Value   *T
	Created bool
}

// ---------- Kind builders ----------

type CreateBuilder[T any, In any] struct {
	base *MutationBuilder[T, In]
	fn   func(ctx context.Context, in In) (*T, error)
}

func (b *CreateBuilder[T, In]) WithResolver(
	fn func(ctx context.Context, in In) (*T, error),
) *CreateBuilder[T, In] {
	b.fn = fn
	return b
}

func (b *CreateBuilder[T, In]) Build() MutationField {
	return &builtMutation{
		name: b.base.name,
		serve: buildMutationField[T, In](b.base, mutationKindCreate,
			func(ctx context.Context, in In, _ presenceSet) (interface{}, error) {
				if b.fn == nil {
					return nil, &MutationError{Code: CodeInternal, Message: "resolver not set"}
				}
				return b.fn(ctx, in)
			},
		),
	}
}

type UpdateBuilder[T any, In any] struct {
	base *MutationBuilder[T, In]
	fn   func(ctx context.Context, p Patch[In]) (*T, error)
}

func (b *UpdateBuilder[T, In]) WithResolver(
	fn func(ctx context.Context, p Patch[In]) (*T, error),
) *UpdateBuilder[T, In] {
	b.fn = fn
	return b
}

func (b *UpdateBuilder[T, In]) Build() MutationField {
	return &builtMutation{
		name: b.base.name,
		serve: buildMutationField[T, In](b.base, mutationKindUpdate,
			func(ctx context.Context, in In, present presenceSet) (interface{}, error) {
				if b.fn == nil {
					return nil, &MutationError{Code: CodeInternal, Message: "resolver not set"}
				}
				return b.fn(ctx, Patch[In]{data: in, present: present})
			},
		),
	}
}

type DeleteBuilder[T any, In any] struct {
	base *MutationBuilder[T, In]
	fn   func(ctx context.Context, in In) (*T, error)
}

func (b *DeleteBuilder[T, In]) WithResolver(
	fn func(ctx context.Context, in In) (*T, error),
) *DeleteBuilder[T, In] {
	b.fn = fn
	return b
}

func (b *DeleteBuilder[T, In]) Build() MutationField {
	return &builtMutation{
		name: b.base.name,
		serve: buildMutationField[T, In](b.base, mutationKindDelete,
			func(ctx context.Context, in In, _ presenceSet) (interface{}, error) {
				if b.fn == nil {
					return nil, &MutationError{Code: CodeInternal, Message: "resolver not set"}
				}
				return b.fn(ctx, in)
			},
		),
	}
}

type ActionBuilder[T any, In any] struct {
	base *MutationBuilder[T, In]
	fn   func(ctx context.Context, in In) (*T, error)
}

func (b *ActionBuilder[T, In]) WithResolver(
	fn func(ctx context.Context, in In) (*T, error),
) *ActionBuilder[T, In] {
	b.fn = fn
	return b
}

func (b *ActionBuilder[T, In]) Build() MutationField {
	return &builtMutation{
		name: b.base.name,
		serve: buildMutationField[T, In](b.base, mutationKindAction,
			func(ctx context.Context, in In, _ presenceSet) (interface{}, error) {
				if b.fn == nil {
					return nil, &MutationError{Code: CodeInternal, Message: "resolver not set"}
				}
				return b.fn(ctx, in)
			},
		),
	}
}

type UpsertBuilder[T any, In any] struct {
	base *MutationBuilder[T, In]
	fn   func(ctx context.Context, p Patch[In]) (Result[T], error)
}

func (b *UpsertBuilder[T, In]) WithResolver(
	fn func(ctx context.Context, p Patch[In]) (Result[T], error),
) *UpsertBuilder[T, In] {
	b.fn = fn
	return b
}

func (b *UpsertBuilder[T, In]) Build() MutationField {
	return &builtMutation{
		name:  b.base.name,
		serve: buildUpsertField[T, In](b.base, b.fn),
	}
}

// ---------- Internal plumbing ----------

type mutationKind int

const (
	mutationKindCreate mutationKind = iota
	mutationKindUpdate
	mutationKindDelete
	mutationKindAction
	mutationKindUpsert
)

type builtMutation struct {
	name  string
	serve *graphql.Field
}

func (m *builtMutation) Name() string          { return m.name }
func (m *builtMutation) Serve() *graphql.Field { return m.serve }

func buildMutationField[T any, In any](
	b *MutationBuilder[T, In],
	kind mutationKind,
	invoke func(ctx context.Context, in In, present presenceSet) (interface{}, error),
) *graphql.Field {
	inputType := buildInputObjectFor[In]()
	outputType := buildOutputType[T]()

	resolve := func(p graphql.ResolveParams) (interface{}, error) {
		ctx := p.Context
		if ctx == nil {
			ctx = context.Background()
		}
		in, present, err := decodeInput[In](p.Args, b.inputName)
		if err != nil {
			return nil, err
		}
		if err := runLifecycle(ctx, &in, present, kind); err != nil {
			return nil, err
		}
		return invoke(ctx, in, present)
	}

	if len(b.middlewares) > 0 {
		wrapped := wrapGraphQLResolver(resolve)
		wrapped = applyMiddlewares(wrapped, b.middlewares)
		resolve = unwrapGraphQLResolver(wrapped)
	}

	return &graphql.Field{
		Description: b.description,
		Type:        outputType,
		Args: graphql.FieldConfigArgument{
			b.inputName: &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(inputType),
			},
		},
		Resolve: resolve,
	}
}

func buildUpsertField[T any, In any](
	b *MutationBuilder[T, In],
	fn func(ctx context.Context, p Patch[In]) (Result[T], error),
) *graphql.Field {
	inputType := buildInputObjectFor[In]()
	payloadType := buildUpsertPayloadType[T](b.name)

	resolve := func(p graphql.ResolveParams) (interface{}, error) {
		ctx := p.Context
		if ctx == nil {
			ctx = context.Background()
		}
		in, present, err := decodeInput[In](p.Args, b.inputName)
		if err != nil {
			return nil, err
		}
		if err := runLifecycle(ctx, &in, present, mutationKindUpsert); err != nil {
			return nil, err
		}
		if fn == nil {
			return nil, &MutationError{Code: CodeInternal, Message: "resolver not set"}
		}
		return fn(ctx, Patch[In]{data: in, present: present})
	}

	if len(b.middlewares) > 0 {
		wrapped := wrapGraphQLResolver(resolve)
		wrapped = applyMiddlewares(wrapped, b.middlewares)
		resolve = unwrapGraphQLResolver(wrapped)
	}

	return &graphql.Field{
		Description: b.description,
		Type:        payloadType,
		Args: graphql.FieldConfigArgument{
			b.inputName: &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(inputType),
			},
		},
		Resolve: resolve,
	}
}

func decodeInput[In any](args map[string]interface{}, inputName string) (In, presenceSet, error) {
	var in In
	raw, ok := args[inputName].(map[string]interface{})
	if !ok {
		return in, nil, &MutationError{
			Code: CodeInvalidInput, Message: "missing or malformed input argument",
		}
	}
	present := make(presenceSet, len(raw))
	for k := range raw {
		present[k] = struct{}{}
	}
	if err := decodeWithPlan(decodePlanFor(reflect.TypeOf(in)), raw, reflect.ValueOf(&in).Elem()); err != nil {
		return in, nil, &MutationError{
			Code: CodeInvalidInput, Message: "failed to parse input", Cause: err,
		}
	}
	return in, present, nil
}

// decodeField holds the precomputed info needed to assign one argument to a
// struct field. setter is non-nil for primitive kinds (fast path); complex
// kinds fall back to setFieldValue which handles nested structs, slices, etc.
type decodeField struct {
	name   string
	index  int
	setter func(fv reflect.Value, raw interface{}) error
}

type decodePlan struct {
	fields []decodeField
}

var decodePlanCache sync.Map // map[reflect.Type]*decodePlan

// decodePlanFor returns the cached field plan for t. Plans are built once per
// type — tag parsing, field enumeration, and setter selection all happen on
// first access, never on a hot request.
func decodePlanFor(t reflect.Type) *decodePlan {
	if t == nil {
		return &decodePlan{}
	}
	if v, ok := decodePlanCache.Load(t); ok {
		return v.(*decodePlan)
	}
	n := t.NumField()
	fields := make([]decodeField, 0, n)
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name := getFieldName(f)
		if name == "-" {
			continue
		}
		fields = append(fields, decodeField{
			name:   name,
			index:  i,
			setter: pickSetter(f.Type),
		})
	}
	plan := &decodePlan{fields: fields}
	actual, _ := decodePlanCache.LoadOrStore(t, plan)
	return actual.(*decodePlan)
}

func decodeWithPlan(plan *decodePlan, raw map[string]interface{}, dst reflect.Value) error {
	for _, f := range plan.fields {
		argValue, exists := raw[f.name]
		if !exists || argValue == nil {
			continue
		}
		fv := dst.Field(f.index)
		if f.setter != nil {
			if err := f.setter(fv, argValue); err != nil {
				return fmt.Errorf("failed to set field %s: %w", f.name, err)
			}
			continue
		}
		if err := setFieldValue(fv, argValue); err != nil {
			return fmt.Errorf("failed to set field %s: %w", f.name, err)
		}
	}
	return nil
}

// pickSetter returns a specialized setter for primitive kinds. Pointers,
// slices, structs, maps, and interfaces fall through to the generic
// reflection-based setFieldValue (nil return).
func pickSetter(t reflect.Type) func(reflect.Value, interface{}) error {
	switch t.Kind() {
	case reflect.String:
		return setString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUint
	case reflect.Float32, reflect.Float64:
		return setFloat
	case reflect.Bool:
		return setBool
	}
	return nil
}

func setString(fv reflect.Value, v interface{}) error {
	if s, ok := v.(string); ok {
		fv.SetString(s)
		return nil
	}
	return fmt.Errorf("expected string, got %T", v)
}

func setInt(fv reflect.Value, v interface{}) error {
	switch x := v.(type) {
	case int:
		fv.SetInt(int64(x))
	case int64:
		fv.SetInt(x)
	case int32:
		fv.SetInt(int64(x))
	case float64:
		fv.SetInt(int64(x))
	default:
		return fmt.Errorf("expected int, got %T", v)
	}
	return nil
}

func setUint(fv reflect.Value, v interface{}) error {
	switch x := v.(type) {
	case uint:
		fv.SetUint(uint64(x))
	case uint64:
		fv.SetUint(x)
	case int:
		if x < 0 {
			return fmt.Errorf("expected non-negative int, got %d", x)
		}
		fv.SetUint(uint64(x))
	case float64:
		if x < 0 {
			return fmt.Errorf("expected non-negative number, got %v", x)
		}
		fv.SetUint(uint64(x))
	default:
		return fmt.Errorf("expected uint, got %T", v)
	}
	return nil
}

func setFloat(fv reflect.Value, v interface{}) error {
	switch x := v.(type) {
	case float64:
		fv.SetFloat(x)
	case float32:
		fv.SetFloat(float64(x))
	case int:
		fv.SetFloat(float64(x))
	default:
		return fmt.Errorf("expected float, got %T", v)
	}
	return nil
}

func setBool(fv reflect.Value, v interface{}) error {
	if b, ok := v.(bool); ok {
		fv.SetBool(b)
		return nil
	}
	return fmt.Errorf("expected bool, got %T", v)
}

func runLifecycle[In any](ctx context.Context, in *In, present presenceSet, kind mutationKind) error {
	if n, ok := any(in).(InputNormalizer); ok {
		n.Normalize()
	}
	if a, ok := any(in).(InputAuthorizer); ok {
		if err := a.Authorize(ctx); err != nil {
			return wrapAs(CodeUnauthorized, err)
		}
	}
	switch kind {
	case mutationKindUpdate, mutationKindUpsert:
		if v, ok := any(in).(PatchInputValidator); ok {
			if err := v.ValidatePatch(ctx, present); err != nil {
				return wrapAs(CodeInvalidInput, err)
			}
			return nil
		}
	}
	if v, ok := any(in).(InputValidator); ok {
		if err := v.Validate(ctx); err != nil {
			return wrapAs(CodeInvalidInput, err)
		}
	}
	return nil
}

// ---------- Schema type helpers ----------

func buildInputObjectFor[In any]() *graphql.InputObject {
	var instance In
	t := reflect.TypeOf(instance)
	if t == nil {
		panic("NewMutation: In type parameter cannot be nil interface")
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("NewMutation: In must be a struct, got %s", t.Kind()))
	}
	name := t.Name()
	if name == "" {
		name = "AnonymousInput"
	}
	if !strings.HasSuffix(name, "Input") {
		name = name + "Input"
	}

	inputTypeRegistryMu.RLock()
	if existing, ok := inputTypeRegistry[name]; ok {
		inputTypeRegistryMu.RUnlock()
		return existing
	}
	inputTypeRegistryMu.RUnlock()

	inputTypeRegistryMu.Lock()
	defer inputTypeRegistryMu.Unlock()
	if existing, ok := inputTypeRegistry[name]; ok {
		return existing
	}

	gen := NewFieldGenerator[any]()
	obj := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   name,
		Fields: gen.generateInputFields(t),
	})
	inputTypeRegistry[name] = obj
	return obj
}

func buildOutputType[T any]() graphql.Output {
	var instance T
	t := reflect.TypeOf(instance)
	if t == nil {
		return graphql.String
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if scalar := scalarForKind(t.Kind()); scalar != nil {
		return scalar
	}
	if t.Kind() == reflect.Slice {
		elem := t.Elem()
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if scalar := scalarForKind(elem.Kind()); scalar != nil {
			return graphql.NewList(scalar)
		}
		return graphql.NewList(buildObjectTypeForReflect(elem))
	}
	return buildObjectTypeForReflect(t)
}

func buildObjectTypeForReflect(t reflect.Type) *graphql.Object {
	name := t.Name()
	if name == "" {
		name = "Anonymous"
	}
	return RegisterObjectType(name, func() *graphql.Object {
		gen := NewFieldGenerator[any]()
		capturedType := t
		return graphql.NewObject(graphql.ObjectConfig{
			Name: name,
			Fields: (graphql.FieldsThunk)(func() graphql.Fields {
				return gen.generateFields(capturedType)
			}),
		})
	})
}

func scalarForKind(k reflect.Kind) graphql.Output {
	switch k {
	case reflect.String:
		return graphql.String
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return graphql.Int
	case reflect.Float32, reflect.Float64:
		return graphql.Float
	case reflect.Bool:
		return graphql.Boolean
	}
	return nil
}

func buildUpsertPayloadType[T any](mutationName string) *graphql.Object {
	payloadName := pascalCase(mutationName) + "Payload"
	resultType := buildOutputType[T]()

	return RegisterObjectType(payloadName, func() *graphql.Object {
		return graphql.NewObject(graphql.ObjectConfig{
			Name: payloadName,
			Fields: graphql.Fields{
				"result": &graphql.Field{
					Type: resultType,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						if r, ok := p.Source.(Result[T]); ok {
							if r.Value == nil {
								return nil, nil
							}
							return *r.Value, nil
						}
						return nil, nil
					},
				},
				"created": &graphql.Field{
					Type: graphql.NewNonNull(graphql.Boolean),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						if r, ok := p.Source.(Result[T]); ok {
							return r.Created, nil
						}
						return false, nil
					},
				},
			},
		})
	})
}

func pascalCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
