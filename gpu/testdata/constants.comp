#version 460
#extension GL_EXT_buffer_reference : require
#extension GL_EXT_scalar_block_layout : require

// Writes its specialization constants to the buffer the push constant points at:
// the defaults declared here, or whatever the pipeline set them to.
layout(constant_id = 0) const bool ENABLED = false;
layout(constant_id = 1) const int OFFSET = -3;
layout(constant_id = 2) const uint COUNT = 7u;
layout(constant_id = 3) const float SCALE = 0.5;

layout(buffer_reference, scalar) writeonly buffer Constants {
    uint enabled;
    int offset;
    uint count;
    float scale;
};

layout(push_constant) uniform PC {
    Constants constants;
} pc;

layout(local_size_x = 1) in;
void main() {
    pc.constants.enabled = ENABLED ? 1u : 0u;
    pc.constants.offset = OFFSET;
    pc.constants.count = COUNT;
    pc.constants.scale = SCALE;
}
