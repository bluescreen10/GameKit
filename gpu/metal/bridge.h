#ifndef PIX_METAL_BRIDGE_H
#define PIX_METAL_BRIDGE_H
#include <stdint.h>
#include <stddef.h>
// Synchronous bridge: pointers in Args are borrowed only for the duration of Call.
// error is a flag, not the message: the text lives in a single buffer reached
// through mbError. Args is built fresh for every command, so carrying kilobytes
// of message space in it would cost an allocation and a memclear per draw.
typedef struct {
    uint64_t u[32];
    double f[8];
    const void *p[4];
    uint32_t error;
} MBArgs;
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
enum {
    MBInit, MBDestroy, MBAlloc, MBFree, MBAddress, MBContents,
    MBTexture, MBView, MBSampler, MBPipeline, MBCompute, MBRelease,
    MBBegin, MBSubmit, MBWait, MBIdle, MBRenderBegin, MBRenderEnd,
    MBSetPipeline, MBViewport, MBScissor, MBDraw, MBIndexed,
    MBIndirect, MBDispatch, MBDispatchIndirect, MBBarrier, MBCopyBuffer,
    MBUpload, MBReadback, MBSurface, MBSwapchain, MBResize, MBAcquire,
    MBPresent, MBPool, MBReadTimes, MBResetTimes, MBTimestamp
};
void *mbCreate(void);
uint64_t mbCall(void *backend, int op, MBArgs *args);
void *mbPointer(uint64_t value);
// mbError returns the message for the call that set MBArgs.error. Like the rest
// of the bridge it assumes calls are externally serialized, so read it before
// making another call.
const char *mbError(void);
#endif

