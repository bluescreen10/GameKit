// Package utils holds small helpers shared across GameKit that belong to no
// particular subsystem.
package utils

import "unsafe"

// ToBytes views any value as the raw bytes of its in-memory representation, without
// copying. It is the way to hand a Go struct to a GPU command as root data:
//
//	root := drawRoot{viewProj: vp, lights: lightsAddr}
//	cmd.Draw(utils.ToBytes(&root), 3, 1, 0, 0)
//
// It takes a pointer and a type parameter rather than `any` deliberately. Boxing into
// an interface would allocate, and this sits on the per-draw path; the generic form
// compiles to an address and a size with nothing at run time. It also keeps the size
// exact — sizeof(T) — rather than whatever a reflected interface reports.
//
// The returned slice ALIASES v; it does not own a copy. Keep v alive and unmodified
// for as long as the bytes are in use. That is trivially true for the intended use,
// where the command records the bytes before the call returns.
//
// The caller is responsible for the layout being one the GPU can read: the struct must
// match what the shader declares, which in practice means explicitly sized fields, no
// pointers, and padding chosen to match the shader's rules rather than left to Go.
func ToBytes[T any](v *T) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(v)), unsafe.Sizeof(*v))
}

// ToBytesSlice views a slice as the raw bytes of its contiguous backing array,
// without copying — ToBytes' counterpart for a buffer upload that carries many
// elements (transforms, indices, indirect args) rather than one push-constant root.
//
// The returned slice ALIASES s; keep s alive and unmodified for as long as the bytes
// are in use. A nil or empty s returns nil.
func ToBytesSlice[T any](s []T) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&s[0])), len(s)*int(unsafe.Sizeof(s[0])))
}
