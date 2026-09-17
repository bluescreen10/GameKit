#ifndef GAMEKIT_METAL_BRIDGE_H
#define GAMEKIT_METAL_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

// Every bridge function is synchronous. Pointer arguments are borrowed only for
// the duration of the call and are never retained by Objective-C.
typedef struct {
    uint64_t value;
    uint64_t auxiliary;
    uint32_t error;
} MBResult;

typedef struct {
    void *value;
    uint32_t error;
} MBPointerResult;

typedef struct {
    const void *shader;
    uint64_t shaderSize;
    const char *entry;
    const char *label;
    uint32_t groupX;
    uint32_t groupY;
    uint32_t groupZ;
} MBComputePipelineDesc;

typedef struct {
    const void *vertexShader;
    uint64_t vertexShaderSize;
    const void *fragmentShader;
    uint64_t fragmentShaderSize;
    const char *vertexEntry;
    const char *fragmentEntry;
    uint32_t topology;
    uint32_t depthFormat;
    uint32_t samples;
    uint32_t cullMode;
    uint32_t frontFaceCW;
    uint32_t depthTest;
    uint32_t depthWrite;
    uint32_t depthCompare;
    uint32_t colorCount;
    uint32_t colorFormats[8];
    uint64_t blend[8];
} MBGraphicsPipelineDesc;

typedef struct {
    uint64_t color[8];
    uint32_t colorCount;
    uint32_t colorLoad[8];
    uint32_t colorStore[8];
    float colorClear[8][4];
    uint64_t depth;
    uint32_t depthLoad;
    uint32_t depthStore;
    uint32_t depthReadOnly;
    float depthClear;
} MBRenderDesc;

void *mbCreate(void);
MBResult mbInit(void *backend);
MBResult mbDestroy(void *backend);
MBResult mbAlloc(void *backend, uint64_t size, uint32_t memory, const char *label);
MBResult mbBufferAddress(void *backend, uint64_t buffer);
MBPointerResult mbBufferContents(void *backend, uint64_t buffer);
MBResult mbReleaseResource(void *backend, uint64_t resource);
MBResult mbCreateTexture(void *backend, uint32_t kind, uint32_t width,
    uint32_t height, uint32_t depth, uint32_t layers, uint32_t mips,
    uint32_t format, uint32_t usage, uint32_t samples, const char *label);
MBResult mbCreateTextureView(void *backend, uint64_t texture, uint32_t kind,
    uint32_t firstMip, uint32_t mipCount, uint32_t firstLayer,
    uint32_t layerCount);
MBResult mbCreateSampler(void *backend, uint32_t minLinear,
    uint32_t magLinear, uint32_t mipLinear, uint32_t addressU,
    uint32_t addressV, uint32_t addressW, uint32_t compare,
    uint32_t maxAnisotropy, const char *label);
MBResult mbCreateComputePipeline(void *backend, const MBComputePipelineDesc *desc);
MBResult mbCreateGraphicsPipeline(void *backend, const MBGraphicsPipelineDesc *desc);

MBResult mbBegin(void *backend);
MBResult mbSubmit(void *backend, uint64_t command);
MBResult mbWait(void *backend, uint64_t fence);
MBResult mbWaitIdle(void *backend);

MBResult mbCreateMetalSurface(void *window);
MBResult mbCreateSwapchain(void *backend, uintptr_t surface, uint32_t width,
    uint32_t height);
MBResult mbResizeSwapchain(void *backend, uint64_t swapchain, uint32_t width,
    uint32_t height);
MBResult mbSwapchainSize(void *backend, uint64_t swapchain);
MBResult mbAcquireNext(void *backend, uint64_t swapchain);
MBResult mbPresent(void *backend, uint64_t swapchain, uint64_t command);

MBResult mbCreateTimestampPool(void *backend, uint32_t count);
MBResult mbReadTimestamps(void *backend, uint64_t pool, uint32_t count,
    uint64_t *timestamps);

MBResult mbBeginRenderPass(void *backend, uint64_t command,
    const MBRenderDesc *desc);
MBResult mbEndRenderPass(void *backend, uint64_t command);
MBResult mbSetPipeline(void *backend, uint64_t command, uint64_t pipeline);
MBResult mbSetViewport(void *backend, uint64_t command, double x, double y,
    double width, double height, double minDepth, double maxDepth);
MBResult mbSetScissor(void *backend, uint64_t command, uint32_t x, uint32_t y,
    uint32_t width, uint32_t height);
MBResult mbSetDepthBias(void *backend, uint64_t command, double bias,
    double slope, double clamp);
MBResult mbDraw(void *backend, uint64_t command, const void *data,
    uint32_t dataSize, uint32_t vertexCount, uint32_t instanceCount,
    uint32_t firstVertex, uint32_t firstInstance);
MBResult mbDrawIndexed(void *backend, uint64_t command, const void *data,
    uint32_t dataSize, uint64_t indexBuffer, uint32_t indexCount,
    uint32_t instanceCount, uint32_t firstIndex, int32_t vertexOffset,
    uint32_t firstInstance, uint32_t indexSize);
MBResult mbDrawIndexedIndirect(void *backend, uint64_t command, const void *data,
    uint32_t dataSize, uint64_t indexBuffer, uint64_t indirectBuffer,
    uint64_t offset, uint32_t drawCount, uint32_t stride, uint32_t indexSize);
MBResult mbDispatch(void *backend, uint64_t command, const void *data,
    uint32_t dataSize, uint32_t x, uint32_t y, uint32_t z);
MBResult mbDispatchIndirect(void *backend, uint64_t command, const void *data,
    uint32_t dataSize, uint64_t buffer, uint64_t offset);
MBResult mbBarrier(void *backend, uint64_t command);
MBResult mbCopyBuffer(void *backend, uint64_t command, uint64_t destination,
    uint64_t source, uint64_t destinationOffset, uint64_t sourceOffset,
    uint64_t size);
MBResult mbCopyBufferToTexture(void *backend, uint64_t command,
    uint64_t texture, uint32_t mip, uint32_t layer, uint64_t buffer,
    uint64_t offset);
MBResult mbCopyTextureToBuffer(void *backend, uint64_t command,
    uint64_t buffer, uint64_t texture, uint32_t mip, uint32_t layer);
MBResult mbResetTimestamps(void *backend, uint64_t command, uint64_t pool,
    uint32_t count);
MBResult mbWriteTimestamp(void *backend, uint64_t command, uint64_t pool,
    uint32_t index, uint32_t stage);

// The bridge is externally serialized. Read this immediately after a result whose
// error field is non-zero, before making another bridge call.
const char *mbError(void);

#endif
