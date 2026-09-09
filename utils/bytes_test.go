package utils

import (
	"testing"
	"unsafe"
)

// TestToBytesViewsTheValueInPlace: the bytes must alias the value, not copy it. A copy
// would silently work in a test and then hand the GPU stale data whenever a caller
// mutated the struct between building it and issuing the draw.
func TestToBytesViewsTheValueInPlace(t *testing.T) {
	v := struct{ A, B uint32 }{A: 1, B: 2}
	b := ToBytes(&v)

	if uintptr(unsafe.Pointer(&b[0])) != uintptr(unsafe.Pointer(&v)) {
		t.Fatal("ToBytes copied instead of aliasing the value")
	}
	v.A = 0xAABBCCDD
	if got := *(*uint32)(unsafe.Pointer(&b[0])); got != 0xAABBCCDD {
		t.Fatalf("mutation through the value was not visible in the bytes: %#x", got)
	}
}

// TestToBytesLengthIsExactlySizeof: the length is what gets pushed as push constants,
// so it has to be the type's size — not its capacity, and not rounded.
func TestToBytesLengthIsExactlySizeof(t *testing.T) {
	type root struct {
		ViewProj [16]float32
		Lights   uint64
		Count    uint32
		_        uint32
	}
	var r root
	if got, want := len(ToBytes(&r)), int(unsafe.Sizeof(r)); got != want {
		t.Fatalf("len = %d, want sizeof = %d", got, want)
	}

	// And a few shapes whose size is easy to get wrong.
	var u8 uint8
	if got := len(ToBytes(&u8)); got != 1 {
		t.Errorf("uint8 len = %d, want 1", got)
	}
	var arr [7]uint32
	if got := len(ToBytes(&arr)); got != 28 {
		t.Errorf("[7]uint32 len = %d, want 28", got)
	}
}

// TestToBytesRoundTrips checks the bytes are the value's actual representation, since
// that is what the shader will reinterpret.
func TestToBytesRoundTrips(t *testing.T) {
	type pair struct{ A, B uint32 }
	in := pair{A: 0x11223344, B: 0x55667788}

	var out pair
	copy(ToBytes(&out), ToBytes(&in))
	if out != in {
		t.Fatalf("round trip gave %+v, want %+v", out, in)
	}
}
