// Package specialization resolves a pipeline descriptor's Constants against the
// specialization constants its shader stages declare, for the backends to pass on.
package specialization

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
)

// Type is the scalar type a specialization constant is declared with.
type Type uint8

const (
	Bool Type = iota
	Int
	Uint
	Float
)

// Declaration is a specialization constant as a shader stage declares it.
type Declaration struct {
	ID   uint32
	Name string
	Type Type
}

// Value is a constant resolved against its declaration: Bits holds the value's 32 bits
// in the declared type — 0 or 1 for a bool, the two's complement of an int, the IEEE
// bits of a float.
type Value struct {
	ID   uint32
	Type Type
	Bits uint32
}

// Resolve matches each constant to the declaration of it, by name or by ID in decimal,
// in any of the stages, and converts its value to the declared type. It fails for a
// constant no stage declares, and for one whose value its type cannot hold. Stages that
// declare the same ID agree on its type, as one shader source compiled per stage does.
func Resolve(constants map[string]float64, stages ...[]Declaration) ([]Value, error) {
	// In key order, so that of several bad constants the same one is reported each time.
	var values []Value
	for _, key := range slices.Sorted(maps.Keys(constants)) {
		declaration, ok := find(key, stages)
		if !ok {
			return nil, fmt.Errorf("no shader stage declares constant %q", key)
		}
		bits, err := convert(constants[key], declaration.Type)
		if err != nil {
			return nil, fmt.Errorf("constant %q: %w", key, err)
		}
		values = append(values, Value{ID: declaration.ID, Type: declaration.Type, Bits: bits})
	}
	return values, nil
}

// find returns the declaration that key names: a declaration's name, or its ID in
// decimal.
func find(key string, stages [][]Declaration) (Declaration, bool) {
	id, err := strconv.ParseUint(key, 10, 32)
	isID := err == nil
	for _, declarations := range stages {
		for _, d := range declarations {
			if d.Name == key || isID && uint64(d.ID) == id {
				return d, true
			}
		}
	}
	return Declaration{}, false
}

func convert(value float64, to Type) (uint32, error) {
	switch to {
	case Bool:
		if value != 0 {
			return 1, nil
		}
		return 0, nil
	case Int:
		if value != math.Trunc(value) || value < math.MinInt32 || value > math.MaxInt32 {
			return 0, fmt.Errorf("%v is not an int32", value)
		}
		return uint32(int32(value)), nil
	case Uint:
		if value != math.Trunc(value) || value < 0 || value > math.MaxUint32 {
			return 0, fmt.Errorf("%v is not a uint32", value)
		}
		return uint32(value), nil
	default:
		return math.Float32bits(float32(value)), nil
	}
}
