package gpu_test

import "math"

// These small test-only helpers keep the GPU conformance suite independent of
// the source repository's glm package.
type mat4 [16]float32
type vec3 [3]float32

func (v vec3) sub(other vec3) vec3 {
	return vec3{v[0] - other[0], v[1] - other[1], v[2] - other[2]}
}

func (v vec3) dot(other vec3) float32 {
	return v[0]*other[0] + v[1]*other[1] + v[2]*other[2]
}

func (v vec3) cross(other vec3) vec3 {
	return vec3{
		v[1]*other[2] - v[2]*other[1],
		v[2]*other[0] - v[0]*other[2],
		v[0]*other[1] - v[1]*other[0],
	}
}

func (v vec3) normalize() vec3 {
	length := float32(math.Sqrt(float64(v.dot(v))))
	return vec3{v[0] / length, v[1] / length, v[2] / length}
}

func perspectiveRH(fovY, aspect, near, far float32) mat4 {
	sinFov, cosFov := math.Sincos(float64(fovY) * 0.5)
	h := float32(cosFov / sinFov)
	w := h / aspect
	r := far / (near - far)
	return mat4{
		w, 0, 0, 0,
		0, h, 0, 0,
		0, 0, r, -1,
		0, 0, r * near, 0,
	}
}

func lookAtRH(eye, center, up vec3) mat4 {
	f := center.sub(eye).normalize()
	s := f.cross(up).normalize()
	u := s.cross(f)
	return mat4{
		s[0], u[0], -f[0], 0,
		s[1], u[1], -f[1], 0,
		s[2], u[2], -f[2], 0,
		-eye.dot(s), -eye.dot(u), eye.dot(f), 1,
	}
}

func mul4x4(a, b mat4) (out mat4) {
	for column := 0; column < 4; column++ {
		for row := 0; row < 4; row++ {
			for k := 0; k < 4; k++ {
				out[column*4+row] += a[k*4+row] * b[column*4+k]
			}
		}
	}
	return out
}
