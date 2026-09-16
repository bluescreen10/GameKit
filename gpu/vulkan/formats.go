package vulkan

// #include "bridge.h"
import "C"

import "github.com/bluescreen10/gamekit/gpu"

// vkFormat maps a gpu.Format to a VkFormat. Formats this backend cannot
// translate (see SupportedFormats) return VK_FORMAT_UNDEFINED.
func vkFormat(f gpu.Format) C.VkFormat {
	switch f {
	case gpu.FormatR8Unorm:
		return C.VK_FORMAT_R8_UNORM
	case gpu.FormatRG8Unorm:
		return C.VK_FORMAT_R8G8_UNORM
	case gpu.FormatRGBA8Unorm:
		return C.VK_FORMAT_R8G8B8A8_UNORM
	case gpu.FormatBGRA8Unorm:
		return C.VK_FORMAT_B8G8R8A8_UNORM
	case gpu.FormatRGBA8Srgb:
		return C.VK_FORMAT_R8G8B8A8_SRGB
	case gpu.FormatBGRA8Srgb:
		return C.VK_FORMAT_B8G8R8A8_SRGB
	case gpu.FormatR8Snorm:
		return C.VK_FORMAT_R8_SNORM
	case gpu.FormatRG8Snorm:
		return C.VK_FORMAT_R8G8_SNORM
	case gpu.FormatRGBA8Snorm:
		return C.VK_FORMAT_R8G8B8A8_SNORM
	case gpu.FormatR8Uint:
		return C.VK_FORMAT_R8_UINT
	case gpu.FormatRG8Uint:
		return C.VK_FORMAT_R8G8_UINT
	case gpu.FormatRGBA8Uint:
		return C.VK_FORMAT_R8G8B8A8_UINT
	case gpu.FormatR8Sint:
		return C.VK_FORMAT_R8_SINT
	case gpu.FormatRG8Sint:
		return C.VK_FORMAT_R8G8_SINT
	case gpu.FormatRGBA8Sint:
		return C.VK_FORMAT_R8G8B8A8_SINT
	case gpu.FormatR16Unorm:
		return C.VK_FORMAT_R16_UNORM
	case gpu.FormatRG16Unorm:
		return C.VK_FORMAT_R16G16_UNORM
	case gpu.FormatRGBA16Unorm:
		return C.VK_FORMAT_R16G16B16A16_UNORM
	case gpu.FormatR16Snorm:
		return C.VK_FORMAT_R16_SNORM
	case gpu.FormatRG16Snorm:
		return C.VK_FORMAT_R16G16_SNORM
	case gpu.FormatRGBA16Snorm:
		return C.VK_FORMAT_R16G16B16A16_SNORM
	case gpu.FormatR16Uint:
		return C.VK_FORMAT_R16_UINT
	case gpu.FormatRG16Uint:
		return C.VK_FORMAT_R16G16_UINT
	case gpu.FormatRGBA16Uint:
		return C.VK_FORMAT_R16G16B16A16_UINT
	case gpu.FormatR16Sint:
		return C.VK_FORMAT_R16_SINT
	case gpu.FormatRG16Sint:
		return C.VK_FORMAT_R16G16_SINT
	case gpu.FormatRGBA16Sint:
		return C.VK_FORMAT_R16G16B16A16_SINT
	case gpu.FormatR16F:
		return C.VK_FORMAT_R16_SFLOAT
	case gpu.FormatRG16F:
		return C.VK_FORMAT_R16G16_SFLOAT
	case gpu.FormatRGBA16F:
		return C.VK_FORMAT_R16G16B16A16_SFLOAT
	case gpu.FormatR32Uint:
		return C.VK_FORMAT_R32_UINT
	case gpu.FormatRG32Uint:
		return C.VK_FORMAT_R32G32_UINT
	case gpu.FormatRGBA32Uint:
		return C.VK_FORMAT_R32G32B32A32_UINT
	case gpu.FormatR32Sint:
		return C.VK_FORMAT_R32_SINT
	case gpu.FormatRG32Sint:
		return C.VK_FORMAT_R32G32_SINT
	case gpu.FormatRGBA32Sint:
		return C.VK_FORMAT_R32G32B32A32_SINT
	case gpu.FormatR32F:
		return C.VK_FORMAT_R32_SFLOAT
	case gpu.FormatRG32F:
		return C.VK_FORMAT_R32G32_SFLOAT
	case gpu.FormatRGBA32F:
		return C.VK_FORMAT_R32G32B32A32_SFLOAT
	case gpu.FormatRGB10A2Unorm:
		return C.VK_FORMAT_A2B10G10R10_UNORM_PACK32
	case gpu.FormatRGB10A2Uint:
		return C.VK_FORMAT_A2B10G10R10_UINT_PACK32
	case gpu.FormatRG11B10F:
		return C.VK_FORMAT_B10G11R11_UFLOAT_PACK32
	case gpu.FormatRGB9E5F:
		return C.VK_FORMAT_E5B9G9R9_UFLOAT_PACK32
	case gpu.FormatDepth16Unorm:
		return C.VK_FORMAT_D16_UNORM
	case gpu.FormatDepth32F:
		return C.VK_FORMAT_D32_SFLOAT
	case gpu.FormatDepth24Stencil8:
		return C.VK_FORMAT_D24_UNORM_S8_UINT
	case gpu.FormatDepth32FStencil8:
		return C.VK_FORMAT_D32_SFLOAT_S8_UINT
	case gpu.FormatStencil8:
		return C.VK_FORMAT_S8_UINT
	case gpu.FormatBC1Unorm:
		return C.VK_FORMAT_BC1_RGBA_UNORM_BLOCK
	case gpu.FormatBC1Srgb:
		return C.VK_FORMAT_BC1_RGBA_SRGB_BLOCK
	case gpu.FormatBC3Unorm:
		return C.VK_FORMAT_BC3_UNORM_BLOCK
	case gpu.FormatBC3Srgb:
		return C.VK_FORMAT_BC3_SRGB_BLOCK
	case gpu.FormatBC4Unorm:
		return C.VK_FORMAT_BC4_UNORM_BLOCK
	case gpu.FormatBC4Snorm:
		return C.VK_FORMAT_BC4_SNORM_BLOCK
	case gpu.FormatBC5Unorm:
		return C.VK_FORMAT_BC5_UNORM_BLOCK
	case gpu.FormatBC5Snorm:
		return C.VK_FORMAT_BC5_SNORM_BLOCK
	case gpu.FormatBC6HFloat:
		return C.VK_FORMAT_BC6H_SFLOAT_BLOCK
	case gpu.FormatBC6HUFloat:
		return C.VK_FORMAT_BC6H_UFLOAT_BLOCK
	case gpu.FormatBC7Unorm:
		return C.VK_FORMAT_BC7_UNORM_BLOCK
	case gpu.FormatBC7Srgb:
		return C.VK_FORMAT_BC7_SRGB_BLOCK
	case gpu.FormatASTC4x4Unorm:
		return C.VK_FORMAT_ASTC_4x4_UNORM_BLOCK
	case gpu.FormatASTC4x4Srgb:
		return C.VK_FORMAT_ASTC_4x4_SRGB_BLOCK
	case gpu.FormatASTC5x5Unorm:
		return C.VK_FORMAT_ASTC_5x5_UNORM_BLOCK
	case gpu.FormatASTC5x5Srgb:
		return C.VK_FORMAT_ASTC_5x5_SRGB_BLOCK
	case gpu.FormatASTC6x6Unorm:
		return C.VK_FORMAT_ASTC_6x6_UNORM_BLOCK
	case gpu.FormatASTC6x6Srgb:
		return C.VK_FORMAT_ASTC_6x6_SRGB_BLOCK
	case gpu.FormatASTC8x8Unorm:
		return C.VK_FORMAT_ASTC_8x8_UNORM_BLOCK
	case gpu.FormatASTC8x8Srgb:
		return C.VK_FORMAT_ASTC_8x8_SRGB_BLOCK
	case gpu.FormatASTC10x10Unorm:
		return C.VK_FORMAT_ASTC_10x10_UNORM_BLOCK
	case gpu.FormatASTC10x10Srgb:
		return C.VK_FORMAT_ASTC_10x10_SRGB_BLOCK
	case gpu.FormatASTC12x12Unorm:
		return C.VK_FORMAT_ASTC_12x12_UNORM_BLOCK
	case gpu.FormatASTC12x12Srgb:
		return C.VK_FORMAT_ASTC_12x12_SRGB_BLOCK
	default:
		return C.VK_FORMAT_UNDEFINED
	}
}

// supportedFormats is the static set this backend accepts. The engine only ever
// runs Vulkan here through KosmicKrisp on macOS (translated onto Metal, not a
// native Vulkan driver), so support tracks the underlying Apple GPU rather than
// the full Vulkan spec: BC and ASTC compressed formats are available (Apple
// Silicon supports both natively), but ETC2/EAC are not (Metal never exposes
// them, on any platform).
var supportedFormats = []gpu.Format{
	gpu.FormatR8Unorm, gpu.FormatRG8Unorm, gpu.FormatRGBA8Unorm, gpu.FormatBGRA8Unorm,
	gpu.FormatRGBA8Srgb, gpu.FormatBGRA8Srgb,
	gpu.FormatR8Snorm, gpu.FormatRG8Snorm, gpu.FormatRGBA8Snorm,
	gpu.FormatR8Uint, gpu.FormatRG8Uint, gpu.FormatRGBA8Uint,
	gpu.FormatR8Sint, gpu.FormatRG8Sint, gpu.FormatRGBA8Sint,
	gpu.FormatR16Unorm, gpu.FormatRG16Unorm, gpu.FormatRGBA16Unorm,
	gpu.FormatR16Snorm, gpu.FormatRG16Snorm, gpu.FormatRGBA16Snorm,
	gpu.FormatR16Uint, gpu.FormatRG16Uint, gpu.FormatRGBA16Uint,
	gpu.FormatR16Sint, gpu.FormatRG16Sint, gpu.FormatRGBA16Sint,
	gpu.FormatR16F, gpu.FormatRG16F, gpu.FormatRGBA16F,
	gpu.FormatR32Uint, gpu.FormatRG32Uint, gpu.FormatRGBA32Uint,
	gpu.FormatR32Sint, gpu.FormatRG32Sint, gpu.FormatRGBA32Sint,
	gpu.FormatR32F, gpu.FormatRG32F, gpu.FormatRGBA32F,
	gpu.FormatRGB10A2Unorm, gpu.FormatRGB10A2Uint, gpu.FormatRG11B10F, gpu.FormatRGB9E5F,
	gpu.FormatDepth16Unorm, gpu.FormatDepth32F, gpu.FormatDepth24Stencil8, gpu.FormatDepth32FStencil8, gpu.FormatStencil8,
	gpu.FormatBC1Unorm, gpu.FormatBC1Srgb, gpu.FormatBC3Unorm, gpu.FormatBC3Srgb,
	gpu.FormatBC4Unorm, gpu.FormatBC4Snorm, gpu.FormatBC5Unorm, gpu.FormatBC5Snorm,
	gpu.FormatBC6HFloat, gpu.FormatBC6HUFloat, gpu.FormatBC7Unorm, gpu.FormatBC7Srgb,
	gpu.FormatASTC4x4Unorm, gpu.FormatASTC4x4Srgb,
	gpu.FormatASTC5x5Unorm, gpu.FormatASTC5x5Srgb,
	gpu.FormatASTC6x6Unorm, gpu.FormatASTC6x6Srgb,
	gpu.FormatASTC8x8Unorm, gpu.FormatASTC8x8Srgb,
	gpu.FormatASTC10x10Unorm, gpu.FormatASTC10x10Srgb,
	gpu.FormatASTC12x12Unorm, gpu.FormatASTC12x12Srgb,
}

// SupportedFormats reports the Format values this backend can create textures
// with. See supportedFormats for why the set is narrower than the full Vulkan spec.
func (b *Backend) SupportedFormats() []gpu.Format { return supportedFormats }
