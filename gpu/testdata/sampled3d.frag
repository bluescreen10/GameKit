#version 460
#extension GL_EXT_buffer_reference : require
#extension GL_EXT_scalar_block_layout : require
#extension GL_EXT_nonuniform_qualifier : require
#extension GL_EXT_shader_explicit_arithmetic_types_int64 : require

// Samples one depth of a 3D texture across the quad. 3D textures live in the same heap
// as 2D ones: binding 0 is declared again, as texture3D, and indexed the same way.
layout(set = 0, binding = 0) uniform texture3D gTextures3D[];
layout(set = 0, binding = 2) uniform sampler gSamplers[];

// textured.vert's root, with the depth to sample at after it.
layout(buffer_reference, scalar) readonly buffer Root {
    uint64_t verts;
    uint tex;
    uint samp;
    float w;
};
layout(push_constant) uniform PC { Root root; } pc;

layout(location = 0) in vec2 vUV;
layout(location = 0) out vec4 outColor;

void main() {
    outColor = texture(sampler3D(gTextures3D[nonuniformEXT(pc.root.tex)], gSamplers[pc.root.samp]), vec3(vUV, pc.root.w));
}
