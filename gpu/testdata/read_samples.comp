#version 460
#extension GL_EXT_buffer_reference : require
#extension GL_EXT_scalar_block_layout : require
#extension GL_EXT_nonuniform_qualifier : require
#extension GL_EXT_shader_explicit_arithmetic_types_int64 : require
#extension GL_EXT_samplerless_texture_functions : require

// Copies every sample of a multisampled colour target and depth buffer into a buffer,
// read through the sampled heap (binding 0) declared for multisampled images: for
// pixel p and sample s, samples[(p.y*size + p.x)*sampleCount + s] holds the colour's
// red and the depth.
layout(set = 0, binding = 0) uniform texture2DMS gTexturesMS[];

layout(buffer_reference, scalar) writeonly buffer Samples {
    vec2 values[];
};

layout(push_constant, scalar) uniform PC {
    Samples samples;
    uint color;
    uint depth;
    uint size;
    uint sampleCount;
} pc;

layout(local_size_x = 8, local_size_y = 8) in;
void main() {
    ivec2 p = ivec2(gl_GlobalInvocationID.xy);
    if (p.x >= int(pc.size) || p.y >= int(pc.size)) {
        return;
    }
    uint first = (uint(p.y) * pc.size + uint(p.x)) * pc.sampleCount;
    for (int s = 0; s < int(pc.sampleCount); s++) {
        float red = texelFetch(gTexturesMS[nonuniformEXT(pc.color)], p, s).r;
        float depth = texelFetch(gTexturesMS[nonuniformEXT(pc.depth)], p, s).r;
        pc.samples.values[first + uint(s)] = vec2(red, depth);
    }
}
