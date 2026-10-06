#version 460
#extension GL_GOOGLE_include_directive : require
#include "common.glsl"

// Outputs the root's tint as it is, alpha included, for testing blend states.
layout(location = 0) out vec4 outColor;
void main() {
    outColor = pc.root.tint;
}
