package ui

// Point is a position in logical pixels relative to a widget's parent.
type Point struct {
	X int
	Y int
}

// Size is a width and height in logical pixels.
type Size struct {
	Width  int
	Height int
}

// IsValid reports whether both dimensions are positive.
func (s Size) IsValid() bool {
	return s.Width > 0 && s.Height > 0
}

// Rect is a widget's position and size in logical pixels.
type Rect struct {
	Position Point
	Size     Size
}
