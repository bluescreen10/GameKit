package test

import "github.com/bluescreen10/gamekit/gpu"

// texelSizes is every format this backend can back with plain CPU memory: one texel's
// byte size for each of the uncompressed formats. Block-compressed formats (BC / ETC2
// / ASTC) are absent on purpose — this backend only stores raw texel bytes, with no
// compression math, so it cannot represent them at all.
var texelSizes = map[gpu.Format]uint32{
	gpu.FormatR8Unorm:  1,
	gpu.FormatR8Snorm:  1,
	gpu.FormatR8Uint:   1,
	gpu.FormatR8Sint:   1,
	gpu.FormatStencil8: 1,

	gpu.FormatRG8Unorm:     2,
	gpu.FormatRG8Snorm:     2,
	gpu.FormatRG8Uint:      2,
	gpu.FormatRG8Sint:      2,
	gpu.FormatR16Unorm:     2,
	gpu.FormatR16Snorm:     2,
	gpu.FormatR16Uint:      2,
	gpu.FormatR16Sint:      2,
	gpu.FormatR16F:         2,
	gpu.FormatDepth16Unorm: 2,

	gpu.FormatRGBA8Unorm:      4,
	gpu.FormatBGRA8Unorm:      4,
	gpu.FormatRGBA8Srgb:       4,
	gpu.FormatBGRA8Srgb:       4,
	gpu.FormatRGBA8Snorm:      4,
	gpu.FormatRGBA8Uint:       4,
	gpu.FormatRGBA8Sint:       4,
	gpu.FormatRG16Unorm:       4,
	gpu.FormatRG16Snorm:       4,
	gpu.FormatRG16Uint:        4,
	gpu.FormatRG16Sint:        4,
	gpu.FormatRG16F:           4,
	gpu.FormatR32Uint:         4,
	gpu.FormatR32Sint:         4,
	gpu.FormatR32F:            4,
	gpu.FormatRGB10A2Unorm:    4,
	gpu.FormatRGB10A2Uint:     4,
	gpu.FormatRG11B10F:        4,
	gpu.FormatRGB9E5F:         4,
	gpu.FormatDepth32F:        4,
	gpu.FormatDepth24Stencil8: 4,

	gpu.FormatRGBA16Unorm: 8,
	gpu.FormatRGBA16Snorm: 8,
	gpu.FormatRGBA16Uint:  8,
	gpu.FormatRGBA16Sint:  8,
	gpu.FormatRGBA16F:     8,
	gpu.FormatRG32Uint:    8,
	gpu.FormatRG32Sint:    8,
	gpu.FormatRG32F:       8,

	gpu.FormatDepth32FStencil8: 8,

	gpu.FormatRGBA32Uint: 16,
	gpu.FormatRGBA32Sint: 16,
	gpu.FormatRGBA32F:    16,
}

// SupportedFormats lists every uncompressed format above — see texelSizes.
func (b *Backend) SupportedFormats() []gpu.Format {
	out := make([]gpu.Format, 0, len(texelSizes))
	for f := range texelSizes {
		out = append(out, f)
	}
	return out
}
