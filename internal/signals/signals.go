package signals

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ── Signal ───────────────────────────────────────────────────────────────────

// Signal[T] represents a single datastar signal. Fields:
//
//	Key   — json tag name, used with ds.Bind and ds.Signals map keys
//	Ref   — "$"+Key, used in JS expressions for ds.On, ds.Text, etc.
//	Value — the signal's current value, read/written at the SSE boundary
//
// Signal[T] fields are transparent to encoding/json: marshaling produces the
// raw value, unmarshaling populates Value directly.
type Signal[T any] struct {
	Key   string
	Ref   string
	Value T
}

func (s *Signal[T]) UnmarshalJSON(data []byte) error { return json.Unmarshal(data, &s.Value) }
func (s Signal[T]) MarshalJSON() ([]byte, error)     { return json.Marshal(s.Value) }

// ── SignalDef ─────────────────────────────────────────────────────────────────

// SignalDef[T] is a read-only box holding a signal struct with Key and Ref
// pre-populated from json tags. Call New() to get a copy you can set values on.
//
// Typical usage:
//
//	type PageSignals struct {
//	    WoodPct Signal[int] `json:"wood_pct"`
//	    ManaPct Signal[int] `json:"mana_pct"`
//	}
//	var sigDef = NewSignalDef[PageSignals]()
//
//	// In a handler — populate values from DB:
//	sigs := sigDef.New()
//	sigs.WoodPct.Value = kingdom.WoodPct
//	pageContent(sigs)
//
//	// In a handler — read values from client:
//	input := &PageSignals{}
//	datastar.ReadSignals(r, input)
//
//	// In a content function — initialise datastar signals:
//	ds.Signals(SignalMap(sigs))
type SignalDef[T any] struct{ zero T }

// New returns a copy of the signal struct with Key and Ref populated and all
// Value fields at their zero value. Set Value fields before passing to content
// functions or MarshalAndPatchSignals.
func (d SignalDef[T]) New() T { return d.zero }

// NewSignalDef populates Key and Ref on every Signal[T] field in T from the
// field's json tag and returns a SignalDef[T]. T must be a struct. Panics at
// startup if any Signal[T] field is missing a json tag.
func NewSignalDef[T any]() SignalDef[T] {
	var zero T
	rt := reflect.TypeOf(zero)
	if rt.Kind() != reflect.Struct {
		panic(fmt.Sprintf("NewSignalDef: %s is not a struct", rt))
	}

	sk := reflect.TypeOf((*signalKeyer)(nil)).Elem()
	rv := reflect.New(rt).Elem()
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !reflect.PointerTo(f.Type).Implements(sk) {
			continue
		}
		tag, ok := f.Tag.Lookup("json")
		if !ok || tag == "" || tag == "-" {
			panic(fmt.Sprintf("NewSignalDef: field %s.%s has no json tag", rt.Name(), f.Name))
		}
		key := strings.Split(tag, ",")[0]
		rv.Field(i).Addr().Interface().(signalKeyer).setKey(key)
	}
	return SignalDef[T]{zero: rv.Interface().(T)}
}

// ── SignalMap ─────────────────────────────────────────────────────────────────

// SignalMap converts a signal struct into a map[string]any for use with
// ds.Signals(). Each Signal[T] field contributes its json tag as the key and
// its Value as the value.
func SignalMap[T any](signals T) map[string]any {
	rt := reflect.TypeOf(signals)
	rv := reflect.ValueOf(signals)
	sv := reflect.TypeOf((*signalValuer)(nil)).Elem()

	m := make(map[string]any, rt.NumField())
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !f.Type.Implements(sv) {
			continue
		}
		key := strings.Split(f.Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			continue
		}
		m[key] = rv.Field(i).Interface().(signalValuer).signalValue()
	}
	return m
}

// ── reflection hooks (unexported) ────────────────────────────────────────────

type signalKeyer interface{ setKey(string) }

func (s *Signal[T]) setKey(key string) {
	s.Key = key
	s.Ref = "$" + key
}

type signalValuer interface{ signalValue() any }

func (s Signal[T]) signalValue() any { return s.Value }
