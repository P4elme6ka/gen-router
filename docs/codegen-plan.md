# gen-router code generation plan

This document describes the intended direction for moving `gen-router` from a reflective/compiled-plan runtime to a code-generated runtime while preserving the current public API as much as possible.

## Goals

1. Keep the current handler API shape:
   - `router.Register(...)`
   - `router.Handler[I, O]`
   - `Input.EndpointPath()`
   - `gen-router` tags on input/output structs
2. Generate very fast input binders.
3. Generate very fast output renderers.
4. Make JSON body parsing/encoding pluggable, with a path toward a codegen-friendly backend.
5. Keep a reflective fallback runtime for development, unsupported types, and incremental adoption.

## Non-goals for the first codegen version

- Full OpenAPI generation in the same first step.
- Replacing the whole runtime immediately.
- Supporting every possible Go type on day one.
- Runtime code generation.

## High-level architecture

The system should have three layers:

### 1. Public API layer

This remains the user-facing contract:

- `router.Register(...)`
- `router.Handler[I, O]`
- input/output structs with tags

This should stay stable.

### 2. Intermediate representation (IR)

The generator should convert handlers and tagged structs into a normalized internal representation.

The IR is the single source of truth for:

- route method/path
- input sources and field types
- output variants and shared fields
- body types
- future OpenAPI metadata

### 3. Runtime execution layer

There will be two implementations:

- generated fast path
- reflective fallback path

`router.Register(...)` should use the generated path when available and fall back to the current reflective/compiled-plan runtime otherwise.

## Discovery strategy

Use static analysis, not runtime reflection, for generation.

Recommended tooling:

- `go/packages`
- `go/types`
- `go/ast`

The generator should inspect selected packages and discover:

- concrete handler types implementing `router.Handler[I, O]`
- concrete input/output types used by those handlers
- `EndpointPath()` return values if they are compile-time constants
- `gen-router` tags on the input/output structs

## Suggested generated artifacts

For each package using handlers, generate a file such as:

- `zz_gen_router_gen.go`

That file can contain:

- a generated binder for each input type
- a generated renderer for each output type
- generated route registration helpers
- a generated registration table for the package

Example generated function shapes:

```go
func bindCreateCustomerInput(req *http.Request) (CreateCustomerInput, error)
func renderCreateCustomerOutput(w http.ResponseWriter, out CreateCustomerOutput) error
```

These functions should not use reflection in the hot path.

## Runtime integration options

### Option A: generated registry looked up by route/handler type

At startup, generated code registers available fast paths into a shared runtime registry.

Pros:
- preserves the current `router.Register(...)` call shape
- minimal user changes

Cons:
- needs a robust generated init/registry mechanism

### Option B: generated package-local register helper

The generator emits package-specific helpers like:

```go
func RegisterGeneratedRoutes(r *router.Router)
```

Pros:
- simple and explicit
- very clear which generated code is being used

Cons:
- user code changes slightly

### Recommendation

Start with Option A plus fallback.
That best preserves the existing API.

## JSON backend direction

Input/output body handling is likely to dominate performance, so the JSON strategy matters.

### Phase 1 recommendation

Keep the generated code structured around an abstraction, but default to stdlib encoding initially.

That means generated code can call small helper interfaces like:

```go
type BodyDecoder interface {
	Decode(data []byte, dst any) error
}

type BodyEncoder interface {
	Encode(w io.Writer, src any) error
}
```

### Phase 2 options

#### Option 1: `easyjson`

Pros:
- mature code generation
- strong performance
- no reflection in hot path

Cons:
- requires generated methods on body types
- user workflow gets more complex

#### Option 2: `segmentio/encoding/json`

Pros:
- faster than stdlib in many cases
- easy integration
- lower workflow complexity

Cons:
- not as strong a codegen story as dedicated generators

#### Option 3: custom codegen for body structs

Pros:
- maximum control
- ideal alignment with `gen-router`

Cons:
- large engineering effort
- correctness burden is high

### Recommendation

Start with:
- generated routing/binding/rendering code
- stdlib or `segmentio/encoding/json` backend behind an abstraction

Later, optionally add support for `easyjson` or your own generated body codecs.

## First supported type set for generated binders

Generated input binders should initially support:

- body: JSON object into a concrete struct
- path/query/header/cookie fields of:
  - `string`
  - `bool`
  - signed ints
  - unsigned ints
  - floats
  - `uuid.UUID`
  - pointers to those scalar types
  - slices of those scalar types for repeated query/header values

That matches the current reflective runtime closely.

## Output generation rules

Generated renderers should preserve current behavior:

- response variants come from `response:XXX`
- shared/root output fields are output fields without `response:XXX`
- shared fields apply to any selected variant
- shared fields do not select a variant
- first matching variant in field order wins
- shared `in:body` fields should be rejected

## Fallback behavior

The current reflective runtime should remain available.

Use fallback when:

- generated code is absent
- a handler/type is unsupported by codegen
- generation is disabled in development

This makes adoption incremental and keeps the project usable throughout the transition.

## Proposed package layout

- `cmd/gen-router/main.go`
  - generator CLI entrypoint
- `internal/codegen/ir`
  - intermediate representation types
- `internal/codegen/discover`
  - package analysis and handler discovery
- `internal/codegen/emit`
  - Go source generation
- `internal/codegen/runtime`
  - optional shared generated runtime helpers/registry

## Incremental roadmap

### Phase 0

Done already:
- stable handler API
- compiled reflective runtime
- benchmark coverage

### Phase 1

Scaffold generator and IR.

Deliverables:
- generator CLI
- IR model
- package loading
- no-op or metadata-only output

### Phase 2

Generate fast input binders.

Deliverables:
- generated parsing for path/query/header/cookie
- generated body decode call sites
- runtime fallback for unsupported cases

### Phase 3

Generate fast output renderers.

Deliverables:
- generated response selection
- generated shared header emission
- generated body write call sites

### Phase 4

Generated registration integration.

Deliverables:
- generated registry or generated registration helper
- `router.Register(...)` prefers generated runtime

### Phase 5

JSON backend optimization.

Deliverables:
- body codec abstraction
- pluggable JSON backend
- optional generated body codecs

## Open questions

1. Should the generator discover all handlers in a package, or only handlers referenced in registration sites?
2. Should generated files be package-local only, or also include a module-wide registry file?
3. Which JSON backend should be the first non-stdlib target?
4. How strict should the generator be for unsupported tags/types: fail hard or partially fall back?

## Recommendation for the next implementation step

The next concrete step should be:

1. add the generator CLI
2. add IR types
3. load packages with `go/packages`
4. emit a metadata summary file or stdout plan

That keeps scope controlled while setting up the codegen foundation correctly.

