//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include "bridge.h"
#include <mach/mach_time.h>

// Metal 4 backend. Residency is explicit (no useResources:) and there is no
// setVertexBytes:/setFragmentBytes: equivalent — per-draw data goes through an
// MTL4ArgumentTable populated with raw GPU addresses instead. See README.md.
//
// This bridge fully syncs every submission (Wait immediately follows Submit/Present,
// see submit()), so there is never more than one command buffer in flight. That lets
// several things be simpler than a pipelined design would allow: the root-struct
// scratch buffer and each command's argument table can be reused across frames
// without double-buffering, because by the time a frame's CPU recording starts, the
// previous frame's GPU work has already completed.
//
// ARC is deliberately not required: each registry entry owns its native objects.
@interface MBResource : NSObject
@property(retain) id object;
@property(retain) id auxiliary;
@property NSUInteger slot;
@property NSUInteger kind; // 1 buffer, 2 texture, 3 sampler, 4 graphics, 5 compute, 6 swapchain, 7 query
@property NSUInteger topology;
@property NSUInteger cull;
@property BOOL clockwise;
@property BOOL depthWrite;
@property MTLSize group;
// Bytes the shader's root argument occupies, from pipeline reflection. MSL rounds a
// struct's size up to its alignment (16 if it holds a vec4 or mat4), so a caller
// packing the same fields more tightly hands the argument table a short root and the
// shader reads past it. Zero when the shader takes no root argument.
@property NSUInteger rootSize;
@property MTLTimestamp cpuStart;
@property MTLTimestamp gpuStart;
@property(retain) NSMutableDictionary *timestampOverrides;
@end
@implementation MBResource
- (void)dealloc { [_object release]; [_auxiliary release]; [_timestampOverrides release]; [super dealloc]; }
@end
@interface MBQuery : NSObject
@property(retain) MBResource *pool;
@property NSUInteger index;
@property NSUInteger stage;
@end
@implementation MBQuery
- (void)dealloc { [_pool release]; [super dealloc]; }
@end
@interface MBCommand : NSObject
@property(retain) id<MTL4CommandBuffer> buffer;
@property(retain) id<MTL4CommandAllocator> allocator;
@property(retain) id<MTL4ArgumentTable> argTable;
@property(retain) id<MTL4RenderCommandEncoder> render;
@property(retain) id<MTL4ComputeCommandEncoder> compute; // also used for copies: MTL4 has no separate blit encoder
@property(retain) MTL4RenderPassDescriptor *pass;
@property(retain) MBResource *pipeline;
@property MTLViewport viewport;
@property MTLScissorRect scissor;
@property BOOL hasViewport;
@property BOOL hasScissor;
@property BOOL readOnlyDepth;
@property(retain) NSMutableArray *pendingQueries;
@property(retain) NSMutableArray *startQueries;
@property(retain) NSMutableArray *endQueries;
// Bump offset into MBDevice.rootArena. Zeroed for free every mbBegin, since MBCommand
// is a fresh object each time; each draw/dispatch claims the next 256-byte slot so
// draws recorded before commit don't overwrite each other's root struct.
@property NSUInteger rootCursor;
@end
@implementation MBCommand
- (void)dealloc { [_buffer release]; [_allocator release]; [_argTable release]; [_render release]; [_compute release]; [_pass release]; [_pipeline release]; [_pendingQueries release]; [_startQueries release]; [_endQueries release]; [super dealloc]; }
@end
@interface MBDevice : NSObject
@property(retain) id<MTLDevice> device;
@property(retain) id<MTL4CommandQueue> queue;
@property(retain) id<MTL4Compiler> compiler;
@property(retain) id<MTLResidencySet> residencySet;
@property BOOL residencyDirty;
@property(retain) NSMutableDictionary *resources;
@property(retain) NSMutableDictionary *commands;
@property(retain) NSMutableDictionary *fences;
@property(retain) id<MTLBuffer> textures;
@property(retain) id<MTLBuffer> samplers;
@property(retain) id<MTLBuffer> timestampScratch;
@property(retain) NSMutableIndexSet *textureSlots;
@property(retain) NSMutableIndexSet *samplerSlots;
// Scratch buffer for root structs — see the file comment on why one buffer, reused
// across frames without double-buffering, is safe here.
@property(retain) id<MTLBuffer> rootArena;
@property uint64_t next;
// MTL4 command buffers carry no waitUntilCompleted/.status/.error/addCompletedHandler:
// of their own — completion is reported through a shared event the queue signals, and
// errors/GPU timestamps arrive via a commit feedback block instead. event is signaled
// with an ever-increasing value per submission; fenceErrors holds any error the
// feedback block captured, keyed by that value, until whoever waits collects it.
@property(retain) id<MTLSharedEvent> event;
@property uint64_t eventValue;
@property(retain) NSMutableDictionary *fenceErrors;
// MTL4 has no automatic hazard tracking between encoders at all — not even between
// two compute encoders opened and closed back to back in the same command buffer.
// One fence, updated when any encoder closes and waited on when any encoder opens,
// orders every encoder in a command buffer against the one before it. Coarse (every
// encoder waits on everything before it, not just what it actually depends on), but
// correct, and this bridge does not pipeline within a frame, so the cost is small.
@property(retain) id<MTLFence> fence;
// The command allocator, command buffer, and argument table are created once and
// reused every frame rather than recreated each mbBegin. That reuse is exactly what
// MTL4's allocator model exists to make cheap — recreating them per frame was
// measured to cost roughly what the classic bridge's whole encode phase costs, wiping
// out the residency-set win this backend was built to capture. Reuse is safe because
// this bridge fully syncs every submission (see the file comment): by the time the
// next mbBegin resets the allocator, the previous frame's GPU work has already
// completed, which is the allocator's own documented precondition for reset.
@property(retain) id<MTL4CommandAllocator> allocator;
@property(retain) id<MTL4CommandBuffer> buffer;
// The shared event this bridge waits on and the commit feedback block that reports
// errors/GPU timestamps are two independent notification paths — Metal gives no
// guarantee the feedback block has actually run by the time a wait on the event
// returns, only that both eventually happen after GPU completion. One reused
// semaphore (safe because, like the allocator above, this bridge never has more than
// one submission outstanding) lets completed() block until the feedback block that
// belongs to THIS submission has actually finished writing fenceErrors/timestampOverrides.
@property(assign) dispatch_semaphore_t feedbackSem;
@property(retain) id<MTL4ArgumentTable> argTable;
@end
@implementation MBDevice
- (void)dealloc {
 [_fences release]; [_commands release]; [_resources release]; [_textures release]; [_samplers release];
 [_timestampScratch release]; [_textureSlots release]; [_samplerSlots release]; [_rootArena release];
 [_residencySet release]; [_compiler release]; [_event release]; [_fenceErrors release]; [_fence release];
 [_allocator release]; [_buffer release]; [_argTable release]; [_queue release]; [_device release];
 if(_feedbackSem) dispatch_release(_feedbackSem);
 [super dealloc];
}
@end
// Legal MTLStages masks for the two fence-bearing encoder types this bridge uses —
// MTLStageAll includes bits outside what a given encoder type can produce/consume,
// which validation rejects.
static const MTLStages kRenderStages = MTLStageVertex|MTLStageFragment|MTLStageTile|MTLStageObject|MTLStageMesh;
static const MTLStages kComputeStages = MTLStageDispatch|MTLStageBlit|MTLStageAccelerationStructure;
static void require(BOOL condition, NSString *message) {
 if (!condition) [NSException raise:@"MetalRHI" format:@"%@",message];
}
static NSString *str(const void *s) {return s ? [NSString stringWithUTF8String:s] : @"";}
// enrol adds one allocation to the residency set and commits it immediately, so the
// allocation is resident before any command buffer can reference it. MTL4 has no
// useResources: at all — this set is the only mechanism, not an optimization over one.
static void enrol(MBDevice *b,id obj) {
 if(!obj) return;
 [b.residencySet addAllocation:obj];
 [b.residencySet commit];
 [b.residencySet requestResidency];
}
static void unenrol(MBDevice *b,id obj) {
 if(!obj) return;
 [b.residencySet removeAllocation:obj];
 [b.residencySet commit];
}
// rootBytes reports how many bytes the shader expects at buffer(0) — the RHI's root
// argument — or 0 if it binds nothing there.
static NSUInteger rootBytes(NSArray<id<MTLBinding>> *bindings) {
 for(id<MTLBinding> x in bindings) {
  if(x.index==0 && x.type==MTLBindingTypeBuffer && x.used)
   return [(id<MTLBufferBinding>)x bufferDataSize];
 }
 return 0;
}
static MBResource *resource(MBDevice *b,uint64_t h,NSUInteger kind) {
 MBResource *r=b.resources[@(h)]; require(r && (!kind || r.kind==kind),@"invalid resource handle or kind"); return r;
}
static uint64_t addWithResidency(MBDevice *b,id obj,NSUInteger kind,BOOL resident) {
 require(obj!=nil,@"Metal resource creation failed"); MBResource *r=[MBResource new]; r.object=obj; r.kind=kind;
 uint64_t h=++b.next; b.resources[@(h)]=r; [r release];
 if(resident && (kind==1||kind==2)) enrol(b,obj);
 return h;
}
static uint64_t add(MBDevice *b,id obj,NSUInteger kind) { return addWithResidency(b,obj,kind,YES); }
// format maps a gpu.Format (see gpu/rhi.go) to its MTLPixelFormat. This is a
// switch, not the old array-indexed-by-enum-value table, because the RHI's
// Format catalog now spans formats this backend can't create (ETC2/EAC are not
// exposed by Metal on any platform) — those fall through to Invalid rather than
// occupying a table slot. Keep in sync with gpu.Format and with
// gpu/metal/metal_darwin.go's supportedFormats.
static MTLPixelFormat format(uint64_t f) {
 switch(f) {
  case 1: return MTLPixelFormatR8Unorm;
  case 2: return MTLPixelFormatRG8Unorm;
  case 3: return MTLPixelFormatRGBA8Unorm;
  case 4: return MTLPixelFormatBGRA8Unorm;
  case 5: return MTLPixelFormatRGBA8Unorm_sRGB;
  case 6: return MTLPixelFormatBGRA8Unorm_sRGB;
  case 7: return MTLPixelFormatR8Snorm;
  case 8: return MTLPixelFormatRG8Snorm;
  case 9: return MTLPixelFormatRGBA8Snorm;
  case 10: return MTLPixelFormatR8Uint;
  case 11: return MTLPixelFormatRG8Uint;
  case 12: return MTLPixelFormatRGBA8Uint;
  case 13: return MTLPixelFormatR8Sint;
  case 14: return MTLPixelFormatRG8Sint;
  case 15: return MTLPixelFormatRGBA8Sint;
  case 16: return MTLPixelFormatR16Unorm;
  case 17: return MTLPixelFormatRG16Unorm;
  case 18: return MTLPixelFormatRGBA16Unorm;
  case 19: return MTLPixelFormatR16Snorm;
  case 20: return MTLPixelFormatRG16Snorm;
  case 21: return MTLPixelFormatRGBA16Snorm;
  case 22: return MTLPixelFormatR16Uint;
  case 23: return MTLPixelFormatRG16Uint;
  case 24: return MTLPixelFormatRGBA16Uint;
  case 25: return MTLPixelFormatR16Sint;
  case 26: return MTLPixelFormatRG16Sint;
  case 27: return MTLPixelFormatRGBA16Sint;
  case 28: return MTLPixelFormatR16Float;
  case 29: return MTLPixelFormatRG16Float;
  case 30: return MTLPixelFormatRGBA16Float;
  case 31: return MTLPixelFormatR32Uint;
  case 32: return MTLPixelFormatRG32Uint;
  case 33: return MTLPixelFormatRGBA32Uint;
  case 34: return MTLPixelFormatR32Sint;
  case 35: return MTLPixelFormatRG32Sint;
  case 36: return MTLPixelFormatRGBA32Sint;
  case 37: return MTLPixelFormatR32Float;
  case 38: return MTLPixelFormatRG32Float;
  case 39: return MTLPixelFormatRGBA32Float;
  case 40: return MTLPixelFormatRGB10A2Unorm;
  case 41: return MTLPixelFormatRGB10A2Uint;
  case 42: return MTLPixelFormatRG11B10Float;
  case 43: return MTLPixelFormatRGB9E5Float;
  case 44: return MTLPixelFormatDepth16Unorm;
  case 45: return MTLPixelFormatDepth32Float;
  case 46: return MTLPixelFormatDepth24Unorm_Stencil8;
  case 47: return MTLPixelFormatDepth32Float_Stencil8;
  case 48: return MTLPixelFormatStencil8;
  case 49: return MTLPixelFormatBC1_RGBA;
  case 50: return MTLPixelFormatBC1_RGBA_sRGB;
  case 51: return MTLPixelFormatBC3_RGBA;
  case 52: return MTLPixelFormatBC3_RGBA_sRGB;
  case 53: return MTLPixelFormatBC4_RUnorm;
  case 54: return MTLPixelFormatBC4_RSnorm;
  case 55: return MTLPixelFormatBC5_RGUnorm;
  case 56: return MTLPixelFormatBC5_RGSnorm;
  case 57: return MTLPixelFormatBC6H_RGBFloat;
  case 58: return MTLPixelFormatBC6H_RGBUfloat;
  case 59: return MTLPixelFormatBC7_RGBAUnorm;
  case 60: return MTLPixelFormatBC7_RGBAUnorm_sRGB;
  // 61-66: ETC2/EAC — not exposed by Metal on any platform.
  case 67: return MTLPixelFormatASTC_4x4_LDR;
  case 68: return MTLPixelFormatASTC_4x4_sRGB;
  case 69: return MTLPixelFormatASTC_5x5_LDR;
  case 70: return MTLPixelFormatASTC_5x5_sRGB;
  case 71: return MTLPixelFormatASTC_6x6_LDR;
  case 72: return MTLPixelFormatASTC_6x6_sRGB;
  case 73: return MTLPixelFormatASTC_8x8_LDR;
  case 74: return MTLPixelFormatASTC_8x8_sRGB;
  case 75: return MTLPixelFormatASTC_10x10_LDR;
  case 76: return MTLPixelFormatASTC_10x10_sRGB;
  case 77: return MTLPixelFormatASTC_12x12_LDR;
  case 78: return MTLPixelFormatASTC_12x12_sRGB;
  default: return MTLPixelFormatInvalid;
 }
}
static MTLTextureType textureType(uint64_t k) {
 static const MTLTextureType ts[]={MTLTextureType2D,MTLTextureType2DArray,MTLTextureTypeCube,MTLTextureTypeCubeArray,MTLTextureType3D};
 require(k<5,@"unknown texture kind"); return ts[k];
}
static NSUInteger slot(MBDevice *b,MBResource *r,BOOL sampler) {
 NSMutableIndexSet *free=sampler ? b.samplerSlots : b.textureSlots;
 NSUInteger i=free.firstIndex; require(i!=NSNotFound,@"bindless heap exhausted"); [free removeIndex:i]; r.slot=i;
 uint64_t *ptr=[(sampler ? b.samplers : b.textures) contents];
 ptr[i]=sampler ? [(id<MTLSamplerState>)r.object gpuResourceID]._impl : [(id<MTLTexture>)r.object gpuResourceID]._impl;
 return i;
}
static void endCompute(MBDevice *b,MBCommand *c) { if(c.compute) { [c.compute updateFence:b.fence afterEncoderStages:kComputeStages]; [c.compute endEncoding]; c.compute=nil; } }
static void outside(MBDevice *b,MBCommand *c) {require(!c.render,@"operation must be outside a render pass"); endCompute(b,c);}
// argTable lazily creates this command's argument table and binds the two heap
// addresses, which are stable for the device's lifetime and so need setting only
// once per table rather than once per draw (unlike the root, slot 0).
// The table itself is created once, in mbInit — see the comment there. c is unused
// but kept in the signature so every call site reads the same regardless of which
// level actually owns the object.
static id<MTL4ArgumentTable> argTable(MBDevice *b,MBCommand *c) {
 return b.argTable;
}
// rootSlot copies data into the shared root arena and returns its GPU address. Each
// call claims a fresh 256-byte-aligned slot so draws recorded earlier in the same
// command buffer keep their own root even though nothing has been submitted yet.
static uint64_t rootSlot(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 const NSUInteger slotSize=256;
 NSUInteger need=c.rootCursor+MAX(size,1);
 if(!b.rootArena || b.rootArena.length<need) {
  id<MTLBuffer> nb=[b.device newBufferWithLength:MAX(need,16384) options:MTLResourceStorageModeShared];
  require(nb!=nil,@"cannot allocate root scratch buffer");
  if(b.rootArena) unenrol(b,b.rootArena);
  b.rootArena=nb; enrol(b,nb);
 }
 NSUInteger off=c.rootCursor;
 if(size) memcpy((uint8_t*)b.rootArena.contents+off,data,size);
 c.rootCursor+=slotSize;
 return b.rootArena.gpuAddress+off;
}
// Render-pass splits implement coarse barriers/timestamps on tile-based GPUs. Preserve
// attachment contents and dynamic state when resuming the logical pass. MTL4 render
// passes have no loadAction/storeAction distinct from the classic MTLRenderPass types
// reused here (MTLRenderPassColorAttachmentDescriptorArray etc.), so the same
// load-preserving reopen still applies — MTL4 changes how work inside the pass is
// bound and synchronized, not the attachment lifecycle.
static void pauseRender(MBDevice *b,MBCommand *c) {
 for(NSUInteger i=0;i<8;i++) if(c.pass.colorAttachments[i].texture) [c.render setColorStoreAction:MTLStoreActionStore atIndex:i];
 if(c.pass.depthAttachment.texture) [c.render setDepthStoreAction:MTLStoreActionStore];
 [c.render updateFence:b.fence afterEncoderStages:kRenderStages];
 [c.render endEncoding]; c.render=nil;
}
static void resumeRender(MBDevice *b,MBCommand *c,BOOL load) {
 MTL4RenderPassDescriptor *d=[[c.pass copy] autorelease];
 for(NSUInteger i=0;i<8;i++) if(d.colorAttachments[i].texture) { if(load) d.colorAttachments[i].loadAction=MTLLoadActionLoad; d.colorAttachments[i].storeAction=MTLStoreActionUnknown; }
 if(d.depthAttachment.texture) { if(load) d.depthAttachment.loadAction=MTLLoadActionLoad; d.depthAttachment.storeAction=MTLStoreActionUnknown; }
 c.render=[c.buffer renderCommandEncoderWithDescriptor:d]; require(c.render!=nil,@"cannot resume render pass");
 [c.render waitForFence:b.fence beforeEncoderStages:kRenderStages];
 [c.render setArgumentTable:argTable(b,c) atStages:MTLRenderStageVertex|MTLRenderStageFragment];
 if(c.hasViewport) [c.render setViewport:c.viewport]; if(c.hasScissor) [c.render setScissorRect:c.scissor];
}
// EXPERIMENTAL/INCOMPLETE (see mbCreateTimestampPool): Metal 4 replaces the classic
// sample-buffer-attachment model this relied on with MTL4CounterHeap and
// writeTimestampWithGranularity:intoHeap:atIndex:, a genuinely different API this
// port has not adopted yet. MBPool always returns an invalid pool for now, so this
// is unreachable — kept only so the shape of the deferred-sampling contract survives
// for whoever picks the query system back up.
// Unlike the classic API, MTL4 can write a timestamp directly into whatever encoder
// is already open, at the exact point requested — no render-pass split needed. This
// bridge doesn't yet map every RHI stage precisely. Fragment is the legal
// "all rendering work has completed" stage and matches the precision of the classic
// bridge, which sampled at an encoder's end.
static void sampleQuery(MBDevice *b,MBCommand *c,MBQuery *q) {
 id<MTL4CounterHeap> heap=q.pool.object;
 if(c.render) {
  [c.render writeTimestampWithGranularity:MTL4TimestampGranularityRelaxed afterStage:MTLRenderStageFragment intoHeap:heap atIndex:q.index];
 } else if(c.compute) {
  [c.compute writeTimestampWithGranularity:MTL4TimestampGranularityRelaxed intoHeap:heap atIndex:q.index];
 } else {
  // No encoder open (e.g. prepareWork ran just before mbBeginRenderPass): a throwaway
  // compute encoder places the timestamp at this exact point in the command buffer's
  // GPU timeline, the same role a dedicated blit-with-sample-buffer played classically.
  id<MTL4ComputeCommandEncoder> e=[c.buffer computeCommandEncoder]; require(e!=nil,@"cannot create compute encoder for timestamp");
  [e waitForFence:b.fence beforeEncoderStages:kComputeStages];
  [e writeTimestampWithGranularity:MTL4TimestampGranularityRelaxed intoHeap:heap atIndex:q.index];
  [e updateFence:b.fence afterEncoderStages:kComputeStages];
  [e endEncoding];
 }
}
// A leading StageNone query and queries with no following work are resolved from
// the command buffer's GPU boundaries when it completes (see submit()'s feedback
// handler) rather than through sampleQuery — see prepareWork below and the cleanup
// in submit(). Otherwise, defer a query until immediately before the next real
// operation, so sampleQuery always has accurate knowledge of which encoder, if any,
// is open at that point.
static void prepareWork(MBDevice *b,MBCommand *c) {
 if(!c.pendingQueries.count) return;
 for(MBQuery *q in c.pendingQueries) {
  if(q.stage==0) [c.startQueries addObject:q];
  else sampleQuery(b,c,q);
 }
 [c.pendingQueries removeAllObjects];
}
// bindings writes the draw/dispatch data into the root arena and binds its address at
// argument-table slot 0. The data belongs to the call rather than to the encoder, so a
// draw that supplies none binds nothing rather than silently inheriting the previous
// draw's — matches the setBytes-based bridge's contract exactly.
static void bindings(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 if(!data || size==0) return;
 require(size<=4096,@"draw/dispatch data exceeds the root arena's per-slot budget");
 // Catch a root argument the shader would read past. MSL rounds a struct's size up
 // to its alignment, so a caller that packs the same fields to 8-byte alignment
 // comes up short — silently, until something perturbs what follows the buffer.
 require(size>=c.pipeline.rootSize,
   ([NSString stringWithFormat:@"draw/dispatch data is %u bytes but the shader's root argument is %lu; pad it to a multiple of 16",
     size,(unsigned long)c.pipeline.rootSize]));
 uint64_t addr=rootSlot(b,c,data,size);
 id<MTL4ArgumentTable> t=argTable(b,c);
 [t setAddress:addr atIndex:0];
}
static void compute(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 require(!c.render && c.pipeline.kind==5,@"dispatch requires a compute pipeline outside a render pass");
 if(!c.compute) {
  c.compute=[c.buffer computeCommandEncoder]; require(c.compute!=nil,@"cannot create compute encoder");
  [c.compute waitForFence:b.fence beforeEncoderStages:kComputeStages];
  [c.compute setArgumentTable:argTable(b,c)];
 }
 [c.compute setComputePipelineState:c.pipeline.object]; bindings(b,c,data,size);
}
static void draw(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 require(c.render && c.pipeline.kind==4,@"draw requires a graphics pipeline and render pass");
 require(!c.readOnlyDepth || !c.pipeline.depthWrite,@"depth-writing pipeline in read-only depth pass");
 [c.render setRenderPipelineState:c.pipeline.object]; [c.render setDepthStencilState:c.pipeline.auxiliary];
 [c.render setCullMode:c.pipeline.cull]; [c.render setFrontFacingWinding:c.pipeline.clockwise?MTLWindingClockwise:MTLWindingCounterClockwise]; bindings(b,c,data,size);
}
// libraryFunction wraps a compiled function for MTL4's descriptor-based pipeline
// creation, which takes an MTL4FunctionDescriptor rather than an id<MTLFunction>
// directly (the function-resolution work is otherwise identical to the classic
// bridge's function() helper).
static NSString *entryName(const void *s) {
 NSString *e=str(s);
 return e.length ? e : @"main0";
}
static MTL4LibraryFunctionDescriptor *libraryFunction(MBDevice *b,const void *data,NSUInteger size,const void *entry) {
 require(data && size,@"empty shader"); NSError *error=nil; id<MTLLibrary> library=nil;
 if(size>=4 && !memcmp(data,"MTLB",4)) {
  dispatch_data_t bytes=dispatch_data_create(data,size,NULL,DISPATCH_DATA_DESTRUCTOR_DEFAULT);
  library=[b.device newLibraryWithData:bytes error:&error]; dispatch_release(bytes);
 } else {
  require(!(size>=4 && *(const uint32_t*)data==0x07230203),@"SPIR-V is not supported; supply MSL source or a metallib");
  NSString *source=[[NSString alloc] initWithBytes:data length:size encoding:NSUTF8StringEncoding];
  require(source!=nil,@"shader is not UTF-8 MSL or a metallib");
  MTLCompileOptions *options=[MTLCompileOptions new]; options.languageVersion=MTLLanguageVersion3_0;
  library=[b.device newLibraryWithSource:source options:options error:&error]; [options release]; [source release];
 }
 require(library!=nil,error.localizedDescription ?: @"shader compilation failed");
 MTL4LibraryFunctionDescriptor *fd=[MTL4LibraryFunctionDescriptor new];
 fd.library=library; fd.name=entryName(entry); [library release];
 return [fd autorelease];
}
static void completed(MBDevice *b,uint64_t value) {
 BOOL ok=[b.event waitUntilSignaledValue:value timeoutMS:30000];
 require(ok,@"GPU command timed out");
 // The event signal and the commit feedback block are two independent notification
 // paths — Metal guarantees both fire after GPU completion, not that one precedes the
 // other. Waiting on the event alone can return before the feedback block (which
 // populates fenceErrors and any pending query's timestampOverrides) has actually
 // run. See MBDevice.feedbackSem.
 long r=dispatch_semaphore_wait(b.feedbackSem,dispatch_time(DISPATCH_TIME_NOW,(int64_t)30*NSEC_PER_SEC));
 require(r==0,@"GPU completion feedback timed out");
 NSError *err=b.fenceErrors[@(value)];
 [b.fenceErrors removeObjectForKey:@(value)];
 require(err==nil,err.localizedDescription ?: @"GPU command failed");
}
static void idle(MBDevice *b) { for(NSNumber *v in b.fences.allValues) completed(b,v.unsignedLongLongValue); [b.fences removeAllObjects]; }
static uint64_t submit(MBDevice *b,uint64_t h,id<CAMetalDrawable> drawable) {
 MBCommand *c=b.commands[@(h)]; require(c!=nil,@"invalid command buffer"); outside(b,c);
 if(c.pendingQueries.count) { if(!c.endQueries) c.endQueries=[NSMutableArray array]; [c.endQueries addObjectsFromArray:c.pendingQueries]; [c.pendingQueries removeAllObjects]; }
 [c.buffer endCommandBuffer];
 // Retain indirectly referenced objects through GPU completion as well.
 NSArray *objects=[b.resources.allValues copy];
 NSArray *starts=[c.startQueries copy],*ends=[c.endQueries copy];
 uint64_t value=++b.eventValue;
 MTL4CommitOptions *opts=[MTL4CommitOptions new];
 [opts addFeedbackHandler:^(id<MTL4CommitFeedback> feedback) {
  if(feedback.error) b.fenceErrors[@(value)]=feedback.error;
  // GPUStartTime/GPUEndTime are host seconds (CACurrentMediaTime's clock). Translate
  // to mach absolute nanoseconds so boundary and hardware-counter samples returned
  // from one query pool remain directly comparable.
  if(starts.count||ends.count) {
   mach_timebase_info_data_t timebase; mach_timebase_info(&timebase);
   long double hostNow=(long double)mach_absolute_time()*timebase.numer/timebase.denom;
   long double hostOrigin=hostNow-(long double)CACurrentMediaTime()*1e9L;
   uint64_t start=feedback.GPUStartTime>0 ? (uint64_t)(hostOrigin+(long double)feedback.GPUStartTime*1e9L) : 0;
   uint64_t end=feedback.GPUEndTime>0 ? (uint64_t)(hostOrigin+(long double)feedback.GPUEndTime*1e9L) : 0;
   for(MBQuery *q in starts) q.pool.timestampOverrides[@(q.index)]=@(start);
   for(MBQuery *q in ends) q.pool.timestampOverrides[@(q.index)]=@(end);
  }
  (void)objects;
  dispatch_semaphore_signal(b.feedbackSem);
 }];
 if(drawable) [b.queue signalDrawable:drawable];
 id<MTL4CommandBuffer> buffers[]={c.buffer};
 [b.queue commit:buffers count:1 options:opts]; [opts release];
 [b.queue signalEvent:b.event value:value];
 if(drawable) [drawable present];
 [objects release]; [starts release]; [ends release];
 b.fences[@(h)]=@(value); [b.commands removeObjectForKey:@(h)]; return h;
}
void *mbCreate(void) { return [MBDevice new]; }

static char mbErrorText[2048];
const char *mbError(void) { return mbErrorText; }

static MBResult mbSuccess(uint64_t value,uint64_t auxiliary) {
 return (MBResult){value,auxiliary,0};
}
static MBResult mbFailure(NSException *e,MBDevice *b,MBCommand *c) {
 snprintf(mbErrorText,sizeof(mbErrorText),"%s",e.reason.UTF8String);
 // Releasing an encoder without endEncoding aborts the process and hides the
 // useful exception. Close anything the failed operation left open first.
 if(c) { if(c.render) { [c.render endEncoding]; c.render=nil; } endCompute(b,c); }
 return (MBResult){0,0,1};
}
static MBCommand *mbCommand(MBDevice *b,uint64_t handle) {
 MBCommand *c=b.commands[@(handle)]; require(c!=nil,@"invalid command buffer"); return c;
}

#define MB_BEGIN(backendValue, commandHandle) \
 @autoreleasepool { \
  MBDevice *b=(MBDevice *)(backendValue); \
  MBCommand *c=nil; \
  @try { \
   uint64_t mbCommandHandle=(commandHandle); \
   if(mbCommandHandle) c=mbCommand(b,mbCommandHandle);
#define MB_END \
  } \
  @catch(NSException *e) { return mbFailure(e,b,c); } \
 }

MBResult mbInit(void *backend) { MB_BEGIN(backend,0) {
  b.device=[MTLCreateSystemDefaultDevice() autorelease]; require(b.device!=nil,@"no Metal device");
  require(b.device.argumentBuffersSupport==MTLArgumentBuffersTier2 && b.device.hasUnifiedMemory,@"requires a unified-memory GPU with Tier 2 argument buffers");
  b.queue=[[b.device newMTL4CommandQueue] autorelease]; require(b.queue!=nil,@"cannot create command queue");
  MTL4CompilerDescriptor *cd=[MTL4CompilerDescriptor new];
  NSError *cerr=nil; b.compiler=[[b.device newCompilerWithDescriptor:cd error:&cerr] autorelease]; [cd release];
  require(b.compiler!=nil,cerr.localizedDescription ?: @"cannot create Metal 4 compiler");
  MTLResidencySetDescriptor *rd=[MTLResidencySetDescriptor new]; rd.label=@"gamekit.residency";
  NSError *rerr=nil; b.residencySet=[[b.device newResidencySetWithDescriptor:rd error:&rerr] autorelease]; [rd release];
  require(b.residencySet!=nil,rerr.localizedDescription ?: @"cannot create residency set");
  [b.queue addResidencySet:b.residencySet];
  b.resources=[NSMutableDictionary dictionary];
  b.commands=[NSMutableDictionary dictionary]; b.fences=[NSMutableDictionary dictionary]; b.fenceErrors=[NSMutableDictionary dictionary];
  b.event=[[b.device newSharedEvent] autorelease]; require(b.event!=nil,@"cannot create shared event");
  b.fence=[[b.device newFence] autorelease]; require(b.fence!=nil,@"cannot create fence");
  b.feedbackSem=dispatch_semaphore_create(0);
  MTL4CommandAllocatorDescriptor *ad=[MTL4CommandAllocatorDescriptor new];
  NSError *aerr=nil; b.allocator=[[b.device newCommandAllocatorWithDescriptor:ad error:&aerr] autorelease]; [ad release];
  require(b.allocator!=nil,aerr.localizedDescription ?: @"cannot create command allocator");
  b.buffer=[[b.device newCommandBuffer] autorelease]; require(b.buffer!=nil,@"cannot create command buffer");
  MTL4ArgumentTableDescriptor *td=[MTL4ArgumentTableDescriptor new]; td.maxBufferBindCount=3;
  NSError *terr=nil; b.argTable=[[b.device newArgumentTableWithDescriptor:td error:&terr] autorelease]; [td release];
  require(b.argTable!=nil,terr.localizedDescription ?: @"cannot create argument table");
  b.textures=[[b.device newBufferWithLength:65536*8 options:MTLResourceStorageModeShared] autorelease];
  b.samplers=[[b.device newBufferWithLength:2048*8 options:MTLResourceStorageModeShared] autorelease];
  require(b.textures && b.samplers,@"cannot allocate argument tables");
  enrol(b,b.textures); enrol(b,b.samplers);
  [b.argTable setAddress:b.textures.gpuAddress atIndex:1];
  [b.argTable setAddress:b.samplers.gpuAddress atIndex:2];
  b.textureSlots=[NSMutableIndexSet indexSetWithIndexesInRange:NSMakeRange(1,65535)];
  b.timestampScratch=[[b.device newBufferWithLength:4 options:MTLResourceStorageModeShared] autorelease];
  enrol(b,b.timestampScratch);
  b.samplerSlots=[NSMutableIndexSet indexSetWithIndexesInRange:NSMakeRange(1,MIN(2048,b.device.maxArgumentBufferSamplerCount)-1)];
  return mbSuccess(1,0);
 } MB_END
}

MBResult mbDestroy(void *backend) { MB_BEGIN(backend,0) {
 [b release]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbAlloc(void *backend,uint64_t size,uint32_t memory,const char *label) { MB_BEGIN(backend,0) {
  require(size>0 && size<=b.device.maxBufferLength && memory<=1,@"invalid buffer allocation");
  id<MTLBuffer> buf=[b.device newBufferWithLength:size options:memory?MTLResourceStorageModePrivate:MTLResourceStorageModeShared];
  buf.label=str(label); uint64_t h=add(b,buf,1); [buf release]; return mbSuccess(h,0);
 } MB_END
}

MBResult mbBufferAddress(void *backend,uint64_t buffer) { MB_BEGIN(backend,0) {
 return mbSuccess([(id<MTLBuffer>)resource(b,buffer,1).object gpuAddress],0);
 } MB_END
}

MBPointerResult mbBufferContents(void *backend,uint64_t buffer) {
 @autoreleasepool {
  MBDevice *b=(MBDevice *)backend;
  @try {
   id<MTLBuffer> buf=resource(b,buffer,1).object;
   return (MBPointerResult){buf.storageMode==MTLStorageModePrivate?NULL:buf.contents,0};
  } @catch(NSException *e) {
   (void)mbFailure(e,b,nil);
   return (MBPointerResult){NULL,1};
  }
 }
}

MBResult mbReleaseResource(void *backend,uint64_t handle) { MB_BEGIN(backend,0) {
  MBResource *r=b.resources[@(handle)]; if(!r) return mbSuccess(0,0);
  if(r.slot) { BOOL s=r.kind==3; ((uint64_t*)[(s?b.samplers:b.textures) contents])[r.slot]=0; [(s?b.samplerSlots:b.textureSlots) addIndex:r.slot]; }
  if(r.kind==1||r.kind==2) unenrol(b,r.object);
  [b.resources removeObjectForKey:@(handle)]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbCreateTexture(void *backend,uint32_t kind,uint32_t width,uint32_t height,uint32_t depth,uint32_t layers,uint32_t mips,uint32_t pixelFormat,uint32_t usage,uint32_t samples,const char *label) { MB_BEGIN(backend,0) {
  require(width && height,@"texture dimensions must be positive");
  MTLTextureDescriptor *d=[MTLTextureDescriptor new]; d.textureType=textureType(kind); d.width=width; d.height=height; d.depth=MAX(depth,1);
  d.arrayLength=MAX(layers,1); if(kind==2||kind==3) {require(layers>=6 && layers%6==0,@"cube layers must be a multiple of six"); d.arrayLength=layers/6;}
  if(kind==0||kind==4) d.arrayLength=1;
  d.mipmapLevelCount=MAX(mips,1); d.pixelFormat=format(pixelFormat); d.sampleCount=MAX(samples,1);
  if(d.sampleCount>1) {require(kind==0 && d.mipmapLevelCount==1,@"MSAA requires a 2D texture with one mip"); d.textureType=MTLTextureType2DMultisample;}
  d.storageMode=MTLStorageModePrivate; d.usage=MTLTextureUsagePixelFormatView;
  if(usage&1) d.usage|=MTLTextureUsageShaderRead;
  if(usage&2) d.usage|=MTLTextureUsageShaderRead|MTLTextureUsageShaderWrite;
  if(usage&12) d.usage|=MTLTextureUsageRenderTarget;
  id<MTLTexture> t=[b.device newTextureWithDescriptor:d]; [d release]; t.label=str(label);
  uint64_t h=add(b,t,2); [t release]; uint64_t index=(usage&3)?slot(b,resource(b,h,2),NO):0; return mbSuccess(h,index);
 } MB_END
}

MBResult mbCreateTextureView(void *backend,uint64_t texture,uint32_t kind,uint32_t firstMip,uint32_t mipCount,uint32_t firstLayer,uint32_t layerCount) { MB_BEGIN(backend,0) {
  id<MTLTexture> t=resource(b,texture,2).object;
  require(mipCount && layerCount && firstMip+mipCount<=t.mipmapLevelCount,@"invalid texture view range");
  id<MTLTexture> v=[t newTextureViewWithPixelFormat:t.pixelFormat textureType:textureType(kind) levels:NSMakeRange(firstMip,mipCount) slices:NSMakeRange(firstLayer,layerCount)];
  uint64_t h=add(b,v,2); [v release]; return mbSuccess(h,slot(b,resource(b,h,2),NO));
 } MB_END
}

MBResult mbCreateSampler(void *backend,uint32_t minLinear,uint32_t magLinear,uint32_t mipLinear,uint32_t addressU,uint32_t addressV,uint32_t addressW,uint32_t compare,uint32_t maxAnisotropy,const char *label) { MB_BEGIN(backend,0) {
  require(addressU<3 && addressV<3 && addressW<3 && compare<8,@"invalid sampler descriptor");
  MTLSamplerDescriptor *d=[MTLSamplerDescriptor new]; d.minFilter=minLinear?MTLSamplerMinMagFilterLinear:MTLSamplerMinMagFilterNearest; d.magFilter=magLinear?MTLSamplerMinMagFilterLinear:MTLSamplerMinMagFilterNearest;
  d.mipFilter=mipLinear?MTLSamplerMipFilterLinear:MTLSamplerMipFilterNearest;
  const MTLSamplerAddressMode modes[]={MTLSamplerAddressModeClampToEdge,MTLSamplerAddressModeRepeat,MTLSamplerAddressModeMirrorRepeat};
  d.sAddressMode=modes[addressU]; d.tAddressMode=modes[addressV]; d.rAddressMode=modes[addressW]; d.compareFunction=compare; d.maxAnisotropy=MAX(maxAnisotropy,1); d.supportArgumentBuffers=YES; d.label=str(label);
  id<MTLSamplerState> s=[b.device newSamplerStateWithDescriptor:d]; [d release]; uint64_t h=add(b,s,3); [s release]; return mbSuccess(h,slot(b,resource(b,h,3),YES));
 } MB_END
}

MBResult mbCreateComputePipeline(void *backend,const MBComputePipelineDesc *v) { MB_BEGIN(backend,0) {
  require(v!=NULL,@"nil compute pipeline descriptor");
  MTL4ComputePipelineDescriptor *d=[MTL4ComputePipelineDescriptor new];
  d.computeFunctionDescriptor=libraryFunction(b,v->shader,v->shaderSize,v->entry);
  d.options=[MTL4PipelineOptions new]; d.options.shaderReflection=MTL4ShaderReflectionBindingInfo; [d.options release];
  NSError *error=nil;
  id<MTLComputePipelineState> ps=[b.compiler newComputePipelineStateWithDescriptor:d compilerTaskOptions:nil error:&error]; [d release];
  require(ps!=nil,error.localizedDescription);
  MTLSize group=MTLSizeMake(MAX(v->groupX,1),MAX(v->groupY,1),MAX(v->groupZ,1));
  require(group.width*group.height*group.depth<=ps.maxTotalThreadsPerThreadgroup,@"workgroup too large");
  uint64_t h=add(b,ps,5); [ps release]; MBResource *r=resource(b,h,5);
  r.group=group; r.rootSize=rootBytes(ps.reflection.bindings); (void)v->label; return mbSuccess(h,0);
 } MB_END
}

MBResult mbCreateGraphicsPipeline(void *backend,const MBGraphicsPipelineDesc *v) { MB_BEGIN(backend,0) {
  require(v!=NULL && v->colorCount<=8,@"invalid graphics pipeline descriptor");
  MTL4RenderPipelineDescriptor *d=[MTL4RenderPipelineDescriptor new];
  d.vertexFunctionDescriptor=libraryFunction(b,v->vertexShader,v->vertexShaderSize,v->vertexEntry); if(v->fragmentShaderSize) d.fragmentFunctionDescriptor=libraryFunction(b,v->fragmentShader,v->fragmentShaderSize,v->fragmentEntry);
  d.rasterSampleCount=MAX(v->samples,1);
  d.options=[MTL4PipelineOptions new]; d.options.shaderReflection=MTL4ShaderReflectionBindingInfo; [d.options release];
  static const MTLBlendFactor factors[]={MTLBlendFactorZero,MTLBlendFactorOne,MTLBlendFactorSourceAlpha,MTLBlendFactorOneMinusSourceAlpha,MTLBlendFactorDestinationAlpha,MTLBlendFactorOneMinusDestinationAlpha};
  for(NSUInteger i=0;i<v->colorCount;i++) {
   MTL4RenderPipelineColorAttachmentDescriptor *t=d.colorAttachments[i]; t.pixelFormat=format(v->colorFormats[i]); uint64_t blend=v->blend[i];
   t.writeMask=blend?((blend>>1)&15):MTLColorWriteMaskAll;
   // RHI mask uses R at bit 0; Metal uses R at bit 3.
   NSUInteger mask=t.writeMask; t.writeMask=((mask&1)<<3)|((mask&2)<<1)|((mask&4)>>1)|((mask&8)>>3);
   t.blendingState=(blend&1)?MTL4BlendStateEnabled:MTL4BlendStateDisabled;
   if(blend&1) {NSUInteger sc=(blend>>5)&15,dc=(blend>>9)&15,sa=(blend>>17)&15,da=(blend>>21)&15; require(sc<6&&dc<6&&sa<6&&da<6,@"invalid blend factor"); t.sourceRGBBlendFactor=factors[sc]; t.destinationRGBBlendFactor=factors[dc]; t.rgbBlendOperation=(blend>>13)&15; t.sourceAlphaBlendFactor=factors[sa]; t.destinationAlphaBlendFactor=factors[da]; t.alphaBlendOperation=(blend>>25)&15;}
  }
  // Metal 4 pipeline state is no longer tied to a depth/stencil attachment format;
  // it lives purely on the render pass descriptor. The descriptor's depth fields
  // still select the MTLDepthStencilState compare/write behavior below.
  NSError *error=nil;
  id<MTLRenderPipelineState> ps=[b.compiler newRenderPipelineStateWithDescriptor:d compilerTaskOptions:nil error:&error]; [d release]; require(ps!=nil,error.localizedDescription);
  uint64_t h=add(b,ps,4); [ps release]; MBResource *r=resource(b,h,4);
  // bindings() pushes the root to both stages, so the larger expectation governs.
  r.rootSize=MAX(rootBytes(ps.reflection.vertexBindings),rootBytes(ps.reflection.fragmentBindings));
  MTLDepthStencilDescriptor *depth=[MTLDepthStencilDescriptor new]; depth.depthCompareFunction=v->depthTest?v->depthCompare:MTLCompareFunctionAlways; depth.depthWriteEnabled=v->depthWrite;
  r.auxiliary=[[b.device newDepthStencilStateWithDescriptor:depth] autorelease]; [depth release];
  const MTLPrimitiveType topologies[]={MTLPrimitiveTypeTriangle,MTLPrimitiveTypeTriangleStrip,MTLPrimitiveTypeLine,MTLPrimitiveTypePoint}; require(v->topology<4&&v->cullMode<3,@"invalid topology or cull mode");
  r.topology=topologies[v->topology]; r.cull=v->cullMode==1?MTLCullModeBack:v->cullMode==2?MTLCullModeFront:MTLCullModeNone; r.clockwise=v->frontFaceCW; r.depthWrite=v->depthWrite; (void)v->depthFormat; return mbSuccess(h,0);
 } MB_END
}

MBResult mbBegin(void *backend) { MB_BEGIN(backend,0) {
  // Reuse the device's allocator/buffer/argument table rather than creating fresh
  // ones — see the property comment on MBDevice.allocator. Safe because this bridge
  // never has two command buffers in flight: reset's precondition (all prior work
  // from this allocator finished) is guaranteed by the full sync every submission
  // already does.
  //
  // TODO: this makes concurrent recording impossible — b.allocator/b.buffer/b.argTable
  // are one shared instance each, so a second Begin before the first Submits/Presents
  // would reset an allocator still backing an open, unsubmitted recording, or call
  // beginCommandBufferWithAllocator: on a buffer already between its own begin/end.
  // Nothing in pix does concurrent recording today (Capture and Render are each a
  // self-contained Begin→Submit→Wait), so this hasn't mattered — but if that changes,
  // the fix is a small pool of allocator/buffer/table triples (one per in-flight
  // recording), not a redesign of the reuse itself.
  require(b.commands.count==0,@"a command buffer from this device is already open");
  [b.allocator reset];
  [b.buffer beginCommandBufferWithAllocator:b.allocator];
  MBCommand *v=[MBCommand new];
  v.allocator=b.allocator; v.buffer=b.buffer; v.argTable=b.argTable;
  uint64_t h=++b.next; b.commands[@(h)]=v; [v release]; return mbSuccess(h,0);
 } MB_END
}

MBResult mbSubmit(void *backend,uint64_t command) { MB_BEGIN(backend,command) {
 return mbSuccess(submit(b,command,nil),0);
 } MB_END
}
MBResult mbWait(void *backend,uint64_t fence) { MB_BEGIN(backend,0) {
 NSNumber *v=b.fences[@(fence)]; if(v) {completed(b,v.unsignedLongLongValue); [b.fences removeObjectForKey:@(fence)];} return mbSuccess(0,0);
 } MB_END
}
MBResult mbWaitIdle(void *backend) { MB_BEGIN(backend,0) {
 idle(b); return mbSuccess(0,0);
 } MB_END
}

MBResult mbBeginRenderPass(void *backend,uint64_t command,const MBRenderDesc *v) { MB_BEGIN(backend,command) {
  require(!c.render,@"nested render pass"); endCompute(b,c); require(v && v->colorCount<=8,@"invalid render pass");
  c.pass=[MTL4RenderPassDescriptor new];
  for(NSUInteger i=0;i<v->colorCount;i++) {
   id<MTLTexture> t=resource(b,v->color[i],2).object; MTLRenderPassColorAttachmentDescriptor *d=c.pass.colorAttachments[i];
   d.texture=t; d.loadAction=v->colorLoad[i]==1?MTLLoadActionClear:v->colorLoad[i]==2?MTLLoadActionLoad:MTLLoadActionDontCare;
   d.storeAction=v->colorStore[i]?MTLStoreActionStore:MTLStoreActionDontCare;
   d.clearColor=MTLClearColorMake(v->colorClear[i][0],v->colorClear[i][1],v->colorClear[i][2],v->colorClear[i][3]);
  }
  if(v->depth) {
   id<MTLTexture> t=resource(b,v->depth,2).object; MTLRenderPassDepthAttachmentDescriptor *d=c.pass.depthAttachment;
   d.texture=t; d.loadAction=v->depthLoad==1?MTLLoadActionClear:v->depthLoad==2?MTLLoadActionLoad:MTLLoadActionDontCare;
   d.storeAction=v->depthStore?MTLStoreActionStore:MTLStoreActionDontCare; d.clearDepth=v->depthClear; c.readOnlyDepth=v->depthReadOnly;
  }
  prepareWork(b,c); resumeRender(b,c,NO);
  return mbSuccess(0,0);
 } MB_END
}

MBResult mbEndRenderPass(void *backend,uint64_t command) { MB_BEGIN(backend,command) {
  require(c.render!=nil,@"no active render pass");
  for(NSUInteger i=0;i<8;i++) if(c.pass.colorAttachments[i].texture) [c.render setColorStoreAction:c.pass.colorAttachments[i].storeAction atIndex:i];
  if(c.pass.depthAttachment.texture) [c.render setDepthStoreAction:c.pass.depthAttachment.storeAction];
  [c.render updateFence:b.fence afterEncoderStages:kRenderStages];
  [c.render endEncoding]; c.render=nil; c.pass=nil; c.hasViewport=NO; c.hasScissor=NO; c.readOnlyDepth=NO; return mbSuccess(0,0);
 } MB_END
}

MBResult mbSetPipeline(void *backend,uint64_t command,uint64_t pipeline) { MB_BEGIN(backend,command) {
 c.pipeline=resource(b,pipeline,0); require(c.pipeline.kind==4||c.pipeline.kind==5,@"invalid pipeline"); return mbSuccess(0,0);
 } MB_END
}
MBResult mbSetViewport(void *backend,uint64_t command,double x,double y,double width,double height,double minDepth,double maxDepth) { MB_BEGIN(backend,command) {
 require(c.render!=nil,@"viewport outside render pass"); c.viewport=(MTLViewport){x,y,width,height,minDepth,maxDepth}; c.hasViewport=YES; [c.render setViewport:c.viewport]; return mbSuccess(0,0);
 } MB_END
}
MBResult mbSetScissor(void *backend,uint64_t command,uint32_t x,uint32_t y,uint32_t width,uint32_t height) { MB_BEGIN(backend,command) {
 require(c.render!=nil,@"scissor outside render pass"); c.scissor=(MTLScissorRect){x,y,width,height}; c.hasScissor=YES; [c.render setScissorRect:c.scissor]; return mbSuccess(0,0);
 } MB_END
}
MBResult mbSetDepthBias(void *backend,uint64_t command,double bias,double slope,double clamp) { MB_BEGIN(backend,command) {
 require(c.render!=nil,@"depth bias outside render pass"); [c.render setDepthBias:bias slopeScale:slope clamp:clamp]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbDraw(void *backend,uint64_t command,const void *data,uint32_t dataSize,uint32_t vertexCount,uint32_t instanceCount,uint32_t firstVertex,uint32_t firstInstance) { MB_BEGIN(backend,command) {
 prepareWork(b,c); draw(b,c,data,dataSize); [c.render drawPrimitives:c.pipeline.topology vertexStart:firstVertex vertexCount:vertexCount instanceCount:instanceCount baseInstance:firstInstance]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbDrawIndexed(void *backend,uint64_t command,const void *data,uint32_t dataSize,uint64_t indexBuffer,uint32_t indexCount,uint32_t instanceCount,uint32_t firstIndex,int32_t vertexOffset,uint32_t firstInstance,uint32_t indexSize) { MB_BEGIN(backend,command) {
  prepareWork(b,c); draw(b,c,data,dataSize);
  // Metal takes a BYTE offset here, unlike Vulkan which takes an element index and
  // derives the stride from the bound index type — so firstIndex must be scaled by
  // the index width rather than a hardcoded 4. MTL4 addresses the index buffer
  // directly rather than binding an object, so the offset folds into the address.
  MTLIndexType it = indexSize==2 ? MTLIndexTypeUInt16 : MTLIndexTypeUInt32;
  id<MTLBuffer> ib=resource(b,indexBuffer,1).object; uint64_t off=(uint64_t)firstIndex*indexSize;
  require(off<=ib.length,@"index offset out of bounds");
  [c.render drawIndexedPrimitives:c.pipeline.topology indexCount:indexCount indexType:it indexBuffer:ib.gpuAddress+off indexBufferLength:ib.length-off instanceCount:instanceCount baseVertex:(NSInteger)vertexOffset baseInstance:firstInstance]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbDrawIndexedIndirect(void *backend,uint64_t command,const void *data,uint32_t dataSize,uint64_t indexBuffer,uint64_t indirectBuffer,uint64_t offset,uint32_t drawCount,uint32_t stride,uint32_t indexSize) { MB_BEGIN(backend,command) {
  prepareWork(b,c); draw(b,c,data,dataSize); id<MTLBuffer> buf=resource(b,indirectBuffer,1).object; require(stride>=20 && !(stride%4) && !(offset%4),@"invalid indirect stride/offset"); require(!drawCount || offset+(drawCount-1)*stride+20<=buf.length,@"indirect draw out of bounds");
  MTLIndexType it = indexSize==2 ? MTLIndexTypeUInt16 : MTLIndexTypeUInt32;
  // Metal has no multi-draw-indirect, so one draw per command is the only option.
  id<MTLBuffer> ib=resource(b,indexBuffer,1).object; uint64_t ibAddr=ib.gpuAddress,ibLen=ib.length,bufAddr=buf.gpuAddress;
  for(NSUInteger i=0;i<drawCount;i++) [c.render drawIndexedPrimitives:c.pipeline.topology indexType:it indexBuffer:ibAddr indexBufferLength:ibLen indirectBuffer:bufAddr+offset+i*stride]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbDispatch(void *backend,uint64_t command,const void *data,uint32_t dataSize,uint32_t x,uint32_t y,uint32_t z) { MB_BEGIN(backend,command) {
 prepareWork(b,c); compute(b,c,data,dataSize); [c.compute dispatchThreadgroups:MTLSizeMake(x,y,z) threadsPerThreadgroup:c.pipeline.group]; return mbSuccess(0,0);
 } MB_END
}
MBResult mbDispatchIndirect(void *backend,uint64_t command,const void *data,uint32_t dataSize,uint64_t buffer,uint64_t offset) { MB_BEGIN(backend,command) {
 prepareWork(b,c); compute(b,c,data,dataSize); id<MTLBuffer> buf=resource(b,buffer,1).object; require(offset%4==0 && offset+12<=buf.length,@"indirect dispatch out of bounds"); [c.compute dispatchThreadgroupsWithIndirectBuffer:buf.gpuAddress+offset threadsPerThreadgroup:c.pipeline.group]; return mbSuccess(0,0);
 } MB_END
}

MBResult mbBarrier(void *backend,uint64_t command) { MB_BEGIN(backend,command) {
  // A render encoder cannot express an all-stage, both-direction intra-pass
  // barrier: Metal only permits pre-raster producers for that form. Split the
  // encoder so fragment/tile writes are stored, fenced, and loaded by the resumed
  // encoder. This is the coarse full barrier expected by the RHI.
  if(c.render) {
   pauseRender(b,c); resumeRender(b,c,YES);
  } else {
   endCompute(b,c);
  } return mbSuccess(0,0);
 } MB_END
}

MBResult mbCopyBuffer(void *backend,uint64_t command,uint64_t destination,uint64_t source,uint64_t destinationOffset,uint64_t sourceOffset,uint64_t size) { MB_BEGIN(backend,command) {
  outside(b,c); prepareWork(b,c); id<MTLBuffer> dst=resource(b,destination,1).object,src=resource(b,source,1).object;
  require(destinationOffset<=dst.length && size<=dst.length-destinationOffset && sourceOffset<=src.length && size<=src.length-sourceOffset,@"buffer copy out of bounds");
  // MTL4 folds blit into the compute encoder — there is no separate encoder type.
  if(!c.compute) { c.compute=[c.buffer computeCommandEncoder]; require(c.compute!=nil,@"cannot create compute encoder"); [c.compute waitForFence:b.fence beforeEncoderStages:kComputeStages]; }
  [c.compute copyFromBuffer:src sourceOffset:sourceOffset toBuffer:dst destinationOffset:destinationOffset size:size]; return mbSuccess(0,0);
 } MB_END
}

static void mbCopyTexture(MBDevice *b,MBCommand *c,uint64_t texture,uint32_t mip,uint32_t layer,uint64_t buffer,uint64_t bufferOffset,BOOL upload) {
  outside(b,c); prepareWork(b,c); id<MTLTexture> t=resource(b,texture,2).object; id<MTLBuffer> buf=resource(b,buffer,1).object;
  require(mip<t.mipmapLevelCount && t.sampleCount==1,@"invalid copy mip or multisampled texture");
  NSUInteger layers=t.arrayLength*((t.textureType==MTLTextureTypeCube||t.textureType==MTLTextureTypeCubeArray)?6:1); require(layer<layers,@"copy layer out of bounds");
  NSUInteger bytes=0; switch(t.pixelFormat) {case MTLPixelFormatR8Unorm:bytes=1;break; case MTLPixelFormatRG8Unorm:bytes=2;break;case MTLPixelFormatRGBA16Float:bytes=8;break;case MTLPixelFormatRGBA32Float:bytes=16;break;default:bytes=4;}
  MTLSize size=MTLSizeMake(MAX(t.width>>mip,1),MAX(t.height>>mip,1),MAX(t.depth>>mip,1)); NSUInteger row=size.width*bytes,img=row*size.height,total=img*size.depth;
  require(bufferOffset<=buf.length && total<=buf.length-bufferOffset,@"texture copy buffer too small");
  // One row per blit permits tightly packed RHI buffers without Metal row-pitch padding.
  if(!c.compute) { c.compute=[c.buffer computeCommandEncoder]; require(c.compute!=nil,@"cannot create compute encoder"); [c.compute waitForFence:b.fence beforeEncoderStages:kComputeStages]; } id<MTL4ComputeCommandEncoder> e=c.compute;
  for(NSUInteger z=0;z<size.depth;z++) for(NSUInteger y=0;y<size.height;y++) {
   NSUInteger offset=bufferOffset+z*img+y*row; MTLOrigin origin=MTLOriginMake(0,y,z); MTLSize line=MTLSizeMake(size.width,1,1);
   if(upload) [e copyFromBuffer:buf sourceOffset:offset sourceBytesPerRow:row sourceBytesPerImage:row sourceSize:line toTexture:t destinationSlice:layer destinationLevel:mip destinationOrigin:origin];
   else [e copyFromTexture:t sourceSlice:layer sourceLevel:mip sourceOrigin:origin sourceSize:line toBuffer:buf destinationOffset:offset destinationBytesPerRow:row destinationBytesPerImage:row];
  }
}

MBResult mbCopyBufferToTexture(void *backend,uint64_t command,uint64_t texture,uint32_t mip,uint32_t layer,uint64_t buffer,uint64_t offset) { MB_BEGIN(backend,command) {
 mbCopyTexture(b,c,texture,mip,layer,buffer,offset,YES); return mbSuccess(0,0);
 } MB_END
}
MBResult mbCopyTextureToBuffer(void *backend,uint64_t command,uint64_t buffer,uint64_t texture,uint32_t mip,uint32_t layer) { MB_BEGIN(backend,command) {
 mbCopyTexture(b,c,texture,mip,layer,buffer,0,NO); return mbSuccess(0,0);
 } MB_END
}

MBResult mbCreateMetalSurface(void *window) { MB_BEGIN(nil,0) {
  require([NSThread isMainThread],@"CreateMetalSurface must run on the main thread"); NSWindow *w=(NSWindow*)window; require(w!=nil,@"nil NSWindow");
  NSView *v=w.contentView; if(![v.layer isKindOfClass:CAMetalLayer.class]) {v.wantsLayer=YES; v.layer=[CAMetalLayer layer];}
  CAMetalLayer *layer=(CAMetalLayer*)v.layer; CGFloat scale=w.backingScaleFactor; CGSize points=v.bounds.size;
  layer.contentsScale=scale; layer.drawableSize=CGSizeMake(points.width*scale,points.height*scale); return mbSuccess((uintptr_t)layer,0);
 } MB_END
}

MBResult mbCreateSwapchain(void *backend,uintptr_t surface,uint32_t width,uint32_t height) { MB_BEGIN(backend,0) {
  CAMetalLayer *layer=(CAMetalLayer*)surface; require([layer isKindOfClass:CAMetalLayer.class],@"surface must be a CAMetalLayer");
  layer.device=b.device; layer.pixelFormat=MTLPixelFormatBGRA8Unorm; layer.framebufferOnly=NO; layer.drawableSize=CGSizeMake(width,height); return mbSuccess(add(b,layer,6),0);
 } MB_END
}
MBResult mbResizeSwapchain(void *backend,uint64_t swapchain,uint32_t width,uint32_t height) { MB_BEGIN(backend,0) {
  MBResource *r=resource(b,swapchain,6); require(!r.auxiliary,@"cannot resize with an acquired drawable"); ((CAMetalLayer*)r.object).drawableSize=CGSizeMake(width,height); return mbSuccess(0,0);
 } MB_END
}
MBResult mbSwapchainSize(void *backend,uint64_t swapchain) { MB_BEGIN(backend,0) {
  CGSize size=((CAMetalLayer*)resource(b,swapchain,6).object).drawableSize; return mbSuccess((uint64_t)size.width,(uint64_t)size.height);
 } MB_END
}
MBResult mbAcquireNext(void *backend,uint64_t swapchain) { MB_BEGIN(backend,0) {
  MBResource *r=resource(b,swapchain,6); require(!r.auxiliary,@"drawable already acquired"); id<CAMetalDrawable> d=[(CAMetalLayer*)r.object nextDrawable]; require(d!=nil,@"no drawable available (window hidden or zero size)"); r.auxiliary=d;
  // A drawable is used only as a render attachment; it is never referenced through
  // a bindless argument table. Keeping it out of the residency set means its
  // acquire/release cycle costs nothing beyond this call.
  uint64_t h=addWithResidency(b,d.texture,2,NO); r.slot=h; return mbSuccess(h,0);
 } MB_END
}
MBResult mbPresent(void *backend,uint64_t swapchain,uint64_t command) { MB_BEGIN(backend,command) {
  MBResource *r=resource(b,swapchain,6); require(r.auxiliary!=nil,@"no acquired drawable");
  resource(b,r.slot,2); uint64_t h=submit(b,command,r.auxiliary);
  [b.resources removeObjectForKey:@(r.slot)]; r.slot=0; r.auxiliary=nil;
  // gamekit reuses mapped frame data and reads counters immediately after Present.
  // Match the synchronous presentation contract of the Vulkan backend.
  NSNumber *v=b.fences[@(h)]; completed(b,v.unsignedLongLongValue);
  [b.fences removeObjectForKey:@(h)]; return mbSuccess(h,0);
 } MB_END
}

MBResult mbCreateTimestampPool(void *backend,uint32_t count) { MB_BEGIN(backend,0) {
  if(!count) return mbSuccess(0,0);
  MTL4CounterHeapDescriptor *d=[MTL4CounterHeapDescriptor new];
  d.type=MTL4CounterHeapTypeTimestamp; d.count=count;
  NSError *error=nil; id<MTL4CounterHeap> heap=[b.device newCounterHeapWithDescriptor:d error:&error]; [d release];
  if(!heap) return mbSuccess(0,0); // unsupported device: degrade as before.
  uint64_t h=add(b,heap,7); MBResource *r=resource(b,h,7); r.auxiliary=[NSMutableIndexSet indexSet]; r.timestampOverrides=[NSMutableDictionary dictionary]; MTLTimestamp cpu,gpu; [b.device sampleTimestamps:&cpu gpuTimestamp:&gpu]; r.cpuStart=cpu; r.gpuStart=gpu; return mbSuccess(h,0);
 } MB_END
}

MBResult mbResetTimestamps(void *backend,uint64_t command,uint64_t pool,uint32_t count) { MB_BEGIN(backend,command) {
  if(!pool) return mbSuccess(0,0); outside(b,c); MBResource *r=resource(b,pool,7); id<MTL4CounterHeap> heap=r.object; require(count<=heap.count,@"query reset out of bounds"); [heap invalidateCounterRange:NSMakeRange(0,count)]; [(NSMutableIndexSet*)r.auxiliary removeIndexesInRange:NSMakeRange(0,count)]; for(NSUInteger i=0;i<count;i++) [r.timestampOverrides removeObjectForKey:@(i)]; return mbSuccess(0,0);
 } MB_END
}
MBResult mbWriteTimestamp(void *backend,uint64_t command,uint64_t pool,uint32_t index,uint32_t stage) { MB_BEGIN(backend,command) {
  if(!pool) return mbSuccess(0,0); MBResource *r=resource(b,pool,7); id<MTL4CounterHeap> heap=r.object; require(index<heap.count,@"query index out of bounds");
  if(!c.pendingQueries) { c.pendingQueries=[NSMutableArray array]; c.startQueries=[NSMutableArray array]; c.endQueries=[NSMutableArray array]; }
  MBQuery *q=[MBQuery new]; q.pool=r; q.index=index; q.stage=stage; [c.pendingQueries addObject:q]; [q release]; [(NSMutableIndexSet*)r.auxiliary addIndex:index]; return mbSuccess(0,0);
 } MB_END
}
MBResult mbReadTimestamps(void *backend,uint64_t pool,uint32_t count,uint64_t *timestamps) { MB_BEGIN(backend,0) {
  MBResource *r=resource(b,pool,7); id<MTL4CounterHeap> heap=r.object; require(count<=heap.count,@"query read out of bounds");
  NSData *data=[heap resolveCounterRange:NSMakeRange(0,count)]; require(data.length>=count*sizeof(MTL4TimestampHeapEntry),@"timestamp resolve failed");
  MTLTimestamp cpu,gpu; [b.device sampleTimestamps:&cpu gpuTimestamp:&gpu];
  mach_timebase_info_data_t timebase; mach_timebase_info(&timebase);
  long double cpuScale=(long double)timebase.numer/timebase.denom;
  require(gpu>r.gpuStart,@"GPU timestamp calibration unavailable");
  long double period=(cpu-r.cpuStart)*cpuScale/(gpu-r.gpuStart);
  const MTL4TimestampHeapEntry *values=data.bytes;
  for(NSUInteger i=0;i<count;i++) {
   NSNumber *override=r.timestampOverrides[@(i)];
   timestamps[i]=override ? override.unsignedLongLongValue : ([(NSIndexSet*)r.auxiliary containsIndex:i] && values[i].timestamp ?
    (uint64_t)((long double)r.cpuStart*cpuScale+((long double)values[i].timestamp-r.gpuStart)*period):0);
  }
  return mbSuccess(0,0);
 } MB_END
}

#undef MB_BEGIN
#undef MB_END
