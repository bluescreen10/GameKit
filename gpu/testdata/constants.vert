#version 460

// Draws a quad over the whole target, or over its left half alone when LEFT_HALF is
// set. The constant is the vertex stage's own: the fragment stage declares others.
layout(constant_id = 4) const bool LEFT_HALF = false;

void main() {
    const vec2 corners[6] = vec2[](
        vec2(0, 0), vec2(1, 0), vec2(1, 1),
        vec2(0, 0), vec2(1, 1), vec2(0, 1));
    vec2 corner = corners[gl_VertexIndex];
    float right = LEFT_HALF ? 0.0 : 1.0;
    gl_Position = vec4(mix(-1.0, right, corner.x), mix(-1.0, 1.0, corner.y), 0.0, 1.0);
}
