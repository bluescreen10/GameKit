#version 460
#extension GL_EXT_nonuniform_qualifier : require
#extension GL_EXT_scalar_block_layout : require

// Writes every voxel of a 3D storage image with its own coordinates, as (x, y, z)/255
// in an 8-bit unorm image — so each byte read back is the coordinate itself. Storage
// images are declared again at binding 1, as image3D.
layout(set = 0, binding = 1) uniform writeonly image3D gImages3D[];

layout(push_constant, scalar) uniform PC {
    uint image;
    uint size;
    uint pad0;
    uint pad1;
} pc;

layout(local_size_x = 4, local_size_y = 4, local_size_z = 4) in;
void main() {
    uvec3 p = gl_GlobalInvocationID;
    if (any(greaterThanEqual(p, uvec3(pc.size)))) {
        return;
    }
    imageStore(gImages3D[nonuniformEXT(pc.image)], ivec3(p), vec4(vec3(p) / 255.0, 1.0));
}
