#version 460

// Fills with the colour its specialization constants give, blue by default.
layout(constant_id = 5) const float RED = 0.0;
layout(constant_id = 6) const float GREEN = 0.0;
layout(constant_id = 7) const float BLUE = 1.0;

layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(RED, GREEN, BLUE, 1.0);
}
