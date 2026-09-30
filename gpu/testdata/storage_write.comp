#version 460
#extension GL_EXT_nonuniform_qualifier : require
#extension GL_EXT_scalar_block_layout : require

// Writes four coloured quadrants into a storage image through the bindless heap's
// storage array (binding 1): top-left red, top-right green, bottom-left blue,
// bottom-right white. The same texture is then sampled through the sampled array,
// with the same index.
layout(set = 0, binding = 1) uniform writeonly image2D gImages[];

layout(push_constant, scalar) uniform PC {
    uint image;
    uint size;
    uint pad0;
    uint pad1;
} pc;

layout(local_size_x = 8, local_size_y = 8) in;
void main() {
    uvec2 p = gl_GlobalInvocationID.xy;
    if (p.x >= pc.size || p.y >= pc.size) {
        return;
    }
    bool left = p.x < pc.size / 2u;
    bool top = p.y < pc.size / 2u;
    vec4 color = vec4(1.0);
    if (top && left) {
        color = vec4(1.0, 0.0, 0.0, 1.0);
    } else if (top) {
        color = vec4(0.0, 1.0, 0.0, 1.0);
    } else if (left) {
        color = vec4(0.0, 0.0, 1.0, 1.0);
    }
    imageStore(gImages[nonuniformEXT(pc.image)], ivec2(p), color);
}
