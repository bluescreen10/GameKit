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
// Bump offset into MBDevice.rootArena. Zeroed for free every MBBegin, since MBCommand
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
// reused every frame rather than recreated each MBBegin. That reuse is exactly what
// MTL4's allocator model exists to make cheap — recreating them per frame was
// measured to cost roughly what the classic bridge's whole encode phase costs, wiping
// out the residency-set win this backend was built to capture. Reuse is safe because
// this bridge fully syncs every submission (see the file comment): by the time the
// next MBBegin resets the allocator, the previous frame's GPU work has already
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
static MTLPixelFormat format(uint64_t f) {
 static const MTLPixelFormat fs[]={MTLPixelFormatInvalid,MTLPixelFormatR8Unorm,MTLPixelFormatRG8Unorm,MTLPixelFormatRGBA8Unorm,MTLPixelFormatRGBA8Unorm_sRGB,MTLPixelFormatBGRA8Unorm,MTLPixelFormatBGRA8Unorm_sRGB,MTLPixelFormatRG16Float,MTLPixelFormatRGBA16Float,MTLPixelFormatR32Float,MTLPixelFormatRGBA32Float,MTLPixelFormatRGB10A2Unorm,MTLPixelFormatDepth32Float,MTLPixelFormatDepth24Unorm_Stencil8};
 require(f<sizeof(fs)/sizeof(fs[0]),@"unknown texture format"); return fs[f];
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
// The table itself is created once, in MBInit — see the comment there. c is unused
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
// EXPERIMENTAL/INCOMPLETE (see MBPool): Metal 4 replaces the classic
// sample-buffer-attachment model this relied on with MTL4CounterHeap and
// writeTimestampWithGranularity:intoHeap:atIndex:, a genuinely different API this
// port has not adopted yet. MBPool always returns an invalid pool for now, so this
// is unreachable — kept only so the shape of the deferred-sampling contract survives
// for whoever picks the query system back up.
// Unlike the classic API, MTL4 can write a timestamp directly into whatever encoder
// is already open, at the exact point requested — no render-pass split needed. This
// bridge doesn't thread the RHI's per-call gpu.Stage bitmask into a precise
// MTLRenderStages value (kRenderStages, "after everything", matches the precision the
// classic bridge always had — it only ever sampled at an encoder's end); a future
// caller wanting per-shader-stage timestamps would need that mapping added here.
static void sampleQuery(MBDevice *b,MBCommand *c,MBQuery *q) {
 id<MTL4CounterHeap> heap=q.pool.object;
 if(c.render) {
  [c.render writeTimestampWithGranularity:MTL4TimestampGranularityRelaxed afterStage:(MTLRenderStages)kRenderStages intoHeap:heap atIndex:q.index];
 } else if(c.compute) {
  [c.compute writeTimestampWithGranularity:MTL4TimestampGranularityRelaxed intoHeap:heap atIndex:q.index];
 } else {
  // No encoder open (e.g. prepareWork ran just before MBRenderBegin): a throwaway
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
void *mbPointer(uint64_t v) { return (void*)(uintptr_t)v; }
// The message for the most recent failed call. Only MBArgs.error says whether it
// is current, so it is never cleared on the success path.
static char mbErrorText[2048];
const char *mbError(void) { return mbErrorText; }
uint64_t mbCall(void *backend,int op,MBArgs *a) {
 @autoreleasepool {
 MBDevice *b=backend; MBCommand *c=nil;
 @try {
 uint64_t *u=a->u; double *f=a->f; const void **p=a->p;
 c=u[30] ? b.commands[@(u[30])] : nil;
 if(u[30]) require(c!=nil,@"invalid command buffer");
 switch(op) {
 case MBInit: {
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
  b.samplerSlots=[NSMutableIndexSet indexSetWithIndexesInRange:NSMakeRange(1,MIN(2048,b.device.maxArgumentBufferSamplerCount)-1)]; return 1;
 }
 case MBDestroy: [b release]; return 0;
 case MBAlloc: {
  require(u[0]>0 && u[0]<=b.device.maxBufferLength && u[1]<=1,@"invalid buffer allocation");
  id<MTLBuffer> buf=[b.device newBufferWithLength:u[0] options:u[1]?MTLResourceStorageModePrivate:MTLResourceStorageModeShared];
  buf.label=str(p[0]); uint64_t h=add(b,buf,1); [buf release]; return h;
 }
 case MBAddress: return [(id<MTLBuffer>)resource(b,u[0],1).object gpuAddress];
 case MBContents: {id<MTLBuffer> buf=resource(b,u[0],1).object; return buf.storageMode==MTLStorageModePrivate?0:(uintptr_t)buf.contents;}
 case MBFree: case MBRelease: {
  MBResource *r=b.resources[@(u[0])]; if(!r) return 0;
  if(r.slot) { BOOL s=r.kind==3; ((uint64_t*)[(s?b.samplers:b.textures) contents])[r.slot]=0; [(s?b.samplerSlots:b.textureSlots) addIndex:r.slot]; }
  if(r.kind==1||r.kind==2) unenrol(b,r.object);
  [b.resources removeObjectForKey:@(u[0])]; return 0;
 }
 case MBTexture: {
  require(u[1] && u[2],@"texture dimensions must be positive");
  MTLTextureDescriptor *d=[MTLTextureDescriptor new]; d.textureType=textureType(u[0]); d.width=u[1]; d.height=u[2]; d.depth=MAX(u[3],1);
  d.arrayLength=MAX(u[4],1); if(u[0]==2||u[0]==3) {require(u[4]>=6 && u[4]%6==0,@"cube layers must be a multiple of six"); d.arrayLength=u[4]/6;}
  if(u[0]==0||u[0]==4) d.arrayLength=1;
  d.mipmapLevelCount=MAX(u[5],1); d.pixelFormat=format(u[6]); d.sampleCount=MAX(u[8],1);
  if(d.sampleCount>1) {require(u[0]==0 && d.mipmapLevelCount==1,@"MSAA requires a 2D texture with one mip"); d.textureType=MTLTextureType2DMultisample;}
  d.storageMode=MTLStorageModePrivate; d.usage=MTLTextureUsagePixelFormatView;
  if(u[7]&1) d.usage|=MTLTextureUsageShaderRead;
  if(u[7]&2) d.usage|=MTLTextureUsageShaderRead|MTLTextureUsageShaderWrite;
  if(u[7]&12) d.usage|=MTLTextureUsageRenderTarget;
  id<MTLTexture> t=[b.device newTextureWithDescriptor:d]; [d release]; t.label=str(p[0]);
  uint64_t h=add(b,t,2); [t release]; if(u[7]&3) u[31]=slot(b,resource(b,h,2),NO); return h;
 }
 case MBView: {
  id<MTLTexture> t=resource(b,u[0],2).object;
  require(u[3] && u[5] && u[2]+u[3]<=t.mipmapLevelCount,@"invalid texture view range");
  id<MTLTexture> v=[t newTextureViewWithPixelFormat:t.pixelFormat textureType:textureType(u[1]) levels:NSMakeRange(u[2],u[3]) slices:NSMakeRange(u[4],u[5])];
  uint64_t h=add(b,v,2); [v release]; u[31]=slot(b,resource(b,h,2),NO); return h;
 }
 case MBSampler: {
  require(u[3]<3 && u[4]<3 && u[5]<3 && u[6]<8,@"invalid sampler descriptor");
  MTLSamplerDescriptor *d=[MTLSamplerDescriptor new]; d.minFilter=u[0]?MTLSamplerMinMagFilterLinear:MTLSamplerMinMagFilterNearest; d.magFilter=u[1]?MTLSamplerMinMagFilterLinear:MTLSamplerMinMagFilterNearest;
  d.mipFilter=u[2]?MTLSamplerMipFilterLinear:MTLSamplerMipFilterNearest;
  const MTLSamplerAddressMode modes[]={MTLSamplerAddressModeClampToEdge,MTLSamplerAddressModeRepeat,MTLSamplerAddressModeMirrorRepeat};
  d.sAddressMode=modes[u[3]]; d.tAddressMode=modes[u[4]]; d.rAddressMode=modes[u[5]]; d.compareFunction=u[6]; d.maxAnisotropy=MAX(u[7],1); d.supportArgumentBuffers=YES; d.label=str(p[0]);
  id<MTLSamplerState> s=[b.device newSamplerStateWithDescriptor:d]; [d release]; uint64_t h=add(b,s,3); [s release]; u[31]=slot(b,resource(b,h,3),YES); return h;
 }
 case MBCompute: {
  MTL4ComputePipelineDescriptor *d=[MTL4ComputePipelineDescriptor new];
  d.computeFunctionDescriptor=libraryFunction(b,p[0],u[0],p[1]);
  d.options=[MTL4PipelineOptions new]; d.options.shaderReflection=MTL4ShaderReflectionBindingInfo; [d.options release];
  NSError *error=nil;
  id<MTLComputePipelineState> ps=[b.compiler newComputePipelineStateWithDescriptor:d compilerTaskOptions:nil error:&error]; [d release];
  require(ps!=nil,error.localizedDescription);
  MTLSize group=MTLSizeMake(MAX(u[1],1),MAX(u[2],1),MAX(u[3],1));
  require(group.width*group.height*group.depth<=ps.maxTotalThreadsPerThreadgroup,@"workgroup too large");
  uint64_t h=add(b,ps,5); [ps release]; MBResource *r=resource(b,h,5);
  r.group=group; r.rootSize=rootBytes(ps.reflection.bindings); return h;
 }
 case MBPipeline: {
  MTL4RenderPipelineDescriptor *d=[MTL4RenderPipelineDescriptor new];
  d.vertexFunctionDescriptor=libraryFunction(b,p[0],u[0],p[2]); if(u[1]) d.fragmentFunctionDescriptor=libraryFunction(b,p[1],u[1],p[3]);
  d.rasterSampleCount=MAX(u[4],1);
  d.options=[MTL4PipelineOptions new]; d.options.shaderReflection=MTL4ShaderReflectionBindingInfo; [d.options release];
  static const MTLBlendFactor factors[]={MTLBlendFactorZero,MTLBlendFactorOne,MTLBlendFactorSourceAlpha,MTLBlendFactorOneMinusSourceAlpha,MTLBlendFactorDestinationAlpha,MTLBlendFactorOneMinusDestinationAlpha};
  for(NSUInteger i=0;i<u[10];i++) {
   MTL4RenderPipelineColorAttachmentDescriptor *t=d.colorAttachments[i]; t.pixelFormat=format(u[11+i]); uint64_t v=u[19+i];
   t.writeMask=v?((v>>1)&15):MTLColorWriteMaskAll;
   // RHI mask uses R at bit 0; Metal uses R at bit 3.
   NSUInteger mask=t.writeMask; t.writeMask=((mask&1)<<3)|((mask&2)<<1)|((mask&4)>>1)|((mask&8)>>3);
   t.blendingState=(v&1)?MTL4BlendStateEnabled:MTL4BlendStateDisabled;
   if(v&1) {NSUInteger sc=(v>>5)&15,dc=(v>>9)&15,sa=(v>>17)&15,da=(v>>21)&15; require(sc<6&&dc<6&&sa<6&&da<6,@"invalid blend factor"); t.sourceRGBBlendFactor=factors[sc]; t.destinationRGBBlendFactor=factors[dc]; t.rgbBlendOperation=(v>>13)&15; t.sourceAlphaBlendFactor=factors[sa]; t.destinationAlphaBlendFactor=factors[da]; t.alphaBlendOperation=(v>>25)&15;}
  }
  // Metal 4 pipeline state is no longer tied to a depth/stencil attachment format —
  // that now lives purely on the render pass descriptor (MBRenderBegin). u[3] here
  // still selects the MTLDepthStencilState's compare/write behavior below.
  NSError *error=nil;
  id<MTLRenderPipelineState> ps=[b.compiler newRenderPipelineStateWithDescriptor:d compilerTaskOptions:nil error:&error]; [d release]; require(ps!=nil,error.localizedDescription);
  uint64_t h=add(b,ps,4); [ps release]; MBResource *r=resource(b,h,4);
  // bindings() pushes the root to both stages, so the larger expectation governs.
  r.rootSize=MAX(rootBytes(ps.reflection.vertexBindings),rootBytes(ps.reflection.fragmentBindings));
  MTLDepthStencilDescriptor *depth=[MTLDepthStencilDescriptor new]; depth.depthCompareFunction=u[7]?u[9]:MTLCompareFunctionAlways; depth.depthWriteEnabled=u[8];
  r.auxiliary=[[b.device newDepthStencilStateWithDescriptor:depth] autorelease]; [depth release];
  const MTLPrimitiveType topologies[]={MTLPrimitiveTypeTriangle,MTLPrimitiveTypeTriangleStrip,MTLPrimitiveTypeLine,MTLPrimitiveTypePoint}; require(u[2]<4&&u[5]<3,@"invalid topology or cull mode");
  r.topology=topologies[u[2]]; r.cull=u[5]==1?MTLCullModeBack:u[5]==2?MTLCullModeFront:MTLCullModeNone; r.clockwise=u[6]; r.depthWrite=u[8]; return h;
 }
 case MBBegin: {
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
  uint64_t h=++b.next; b.commands[@(h)]=v; [v release]; return h;
 }
 case MBSubmit: return submit(b,u[0],nil);
 case MBWait: {NSNumber *v=b.fences[@(u[0])]; if(v) {completed(b,v.unsignedLongLongValue); [b.fences removeObjectForKey:@(u[0])];} return 0;}
 case MBIdle: idle(b); return 0;
 case MBRenderBegin: {
  require(!c.render,@"nested render pass"); endCompute(b,c); MBRenderDesc *v=(MBRenderDesc*)p[0]; require(v && v->colorCount<=8,@"invalid render pass");
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
  return 0;
 }
 case MBRenderEnd: require(c.render!=nil,@"no active render pass");
  for(NSUInteger i=0;i<8;i++) if(c.pass.colorAttachments[i].texture) [c.render setColorStoreAction:c.pass.colorAttachments[i].storeAction atIndex:i];
  if(c.pass.depthAttachment.texture) [c.render setDepthStoreAction:c.pass.depthAttachment.storeAction];
  [c.render updateFence:b.fence afterEncoderStages:kRenderStages];
  [c.render endEncoding]; c.render=nil; c.pass=nil; c.hasViewport=NO; c.hasScissor=NO; c.readOnlyDepth=NO; return 0;
 case MBSetPipeline: c.pipeline=resource(b,u[0],0); require(c.pipeline.kind==4||c.pipeline.kind==5,@"invalid pipeline"); return 0;
 case MBViewport: require(c.render!=nil,@"viewport outside render pass"); c.viewport=(MTLViewport){f[0],f[1],f[2],f[3],f[4],f[5]}; c.hasViewport=YES; [c.render setViewport:c.viewport]; return 0;
 case MBScissor: require(c.render!=nil,@"scissor outside render pass"); c.scissor=(MTLScissorRect){u[0],u[1],u[2],u[3]}; c.hasScissor=YES; [c.render setScissorRect:c.scissor]; return 0;
 case MBDraw: prepareWork(b,c); draw(b,c,a->p[0],(uint32_t)u[29]); [c.render drawPrimitives:c.pipeline.topology vertexStart:u[2] vertexCount:u[0] instanceCount:u[1] baseInstance:u[3]]; return 0;
 case MBIndexed: {prepareWork(b,c); draw(b,c,a->p[0],(uint32_t)u[29]);
  // Metal takes a BYTE offset here, unlike Vulkan which takes an element index and
  // derives the stride from the bound index type — so firstIndex must be scaled by
  // the index width rather than a hardcoded 4. MTL4 addresses the index buffer
  // directly rather than binding an object, so the offset folds into the address.
  MTLIndexType it = u[6]==2 ? MTLIndexTypeUInt16 : MTLIndexTypeUInt32;
  id<MTLBuffer> ib=resource(b,u[0],1).object; uint64_t off=u[3]*u[6];
  require(off<=ib.length,@"index offset out of bounds");
  [c.render drawIndexedPrimitives:c.pipeline.topology indexCount:u[1] indexType:it indexBuffer:ib.gpuAddress+off indexBufferLength:ib.length-off instanceCount:u[2] baseVertex:(NSInteger)u[4] baseInstance:u[5]]; return 0;}
 case MBIndirect: {
  prepareWork(b,c); draw(b,c,a->p[0],(uint32_t)u[29]); id<MTLBuffer> buf=resource(b,u[1],1).object; require(u[4]>=20 && !(u[4]%4) && !(u[2]%4),@"invalid indirect stride/offset"); require(!u[3] || u[2]+(u[3]-1)*u[4]+20<=buf.length,@"indirect draw out of bounds");
  MTLIndexType it = u[5]==2 ? MTLIndexTypeUInt16 : MTLIndexTypeUInt32;
  // Metal has no multi-draw-indirect, so one draw per command is the only option.
  id<MTLBuffer> ib=resource(b,u[0],1).object; uint64_t ibAddr=ib.gpuAddress,ibLen=ib.length,bufAddr=buf.gpuAddress;
  for(NSUInteger i=0;i<u[3];i++) [c.render drawIndexedPrimitives:c.pipeline.topology indexType:it indexBuffer:ibAddr indexBufferLength:ibLen indirectBuffer:bufAddr+u[2]+i*u[4]]; return 0;
 }
 case MBDispatch: prepareWork(b,c); compute(b,c,a->p[0],(uint32_t)u[29]); [c.compute dispatchThreadgroups:MTLSizeMake(u[0],u[1],u[2]) threadsPerThreadgroup:c.pipeline.group]; return 0;
 case MBDispatchIndirect: {prepareWork(b,c); compute(b,c,a->p[0],(uint32_t)u[29]); id<MTLBuffer> buf=resource(b,u[0],1).object; require(u[1]%4==0 && u[1]+12<=buf.length,@"indirect dispatch out of bounds"); [c.compute dispatchThreadgroupsWithIndirectBuffer:buf.gpuAddress+u[1] threadsPerThreadgroup:c.pipeline.group]; return 0;}
 case MBBarrier:
  // Metal 4 has no implicit hazard tracking (that is what the residency set replaced
  // useResources: for, not synchronization), so a barrier command must itself stop
  // any in-flight GPU work on this encoder from racing what follows. An intra-pass,
  // all-stages-both-directions barrier is the conservative, always-correct
  // translation of "barrier" as this RHI's callers use it: a full stop, not a
  // fine-grained producer/consumer pair.
  if(c.render) {
   [c.render barrierAfterEncoderStages:kRenderStages beforeEncoderStages:kRenderStages visibilityOptions:0];
  } else {
   endCompute(b,c);
  } return 0;
 case MBCopyBuffer: {
  outside(b,c); prepareWork(b,c); id<MTLBuffer> dst=resource(b,u[0],1).object,src=resource(b,u[1],1).object;
  require(u[2]<=dst.length && u[4]<=dst.length-u[2] && u[3]<=src.length && u[4]<=src.length-u[3],@"buffer copy out of bounds");
  // MTL4 folds blit into the compute encoder — there is no separate encoder type.
  if(!c.compute) { c.compute=[c.buffer computeCommandEncoder]; require(c.compute!=nil,@"cannot create compute encoder"); [c.compute waitForFence:b.fence beforeEncoderStages:kComputeStages]; }
  [c.compute copyFromBuffer:src sourceOffset:u[3] toBuffer:dst destinationOffset:u[2] size:u[4]]; return 0;
 }
 case MBUpload: case MBReadback: {
  outside(b,c); prepareWork(b,c); id<MTLTexture> t=resource(b,u[0],2).object; id<MTLBuffer> buf=resource(b,u[3],1).object;
  require(u[1]<t.mipmapLevelCount && t.sampleCount==1,@"invalid copy mip or multisampled texture");
  NSUInteger layers=t.arrayLength*((t.textureType==MTLTextureTypeCube||t.textureType==MTLTextureTypeCubeArray)?6:1); require(u[2]<layers,@"copy layer out of bounds");
  NSUInteger bytes=0; switch(t.pixelFormat) {case MTLPixelFormatR8Unorm:bytes=1;break; case MTLPixelFormatRG8Unorm:bytes=2;break;case MTLPixelFormatRGBA16Float:bytes=8;break;case MTLPixelFormatRGBA32Float:bytes=16;break;default:bytes=4;}
  MTLSize size=MTLSizeMake(MAX(t.width>>u[1],1),MAX(t.height>>u[1],1),MAX(t.depth>>u[1],1)); NSUInteger row=size.width*bytes,img=row*size.height,total=img*size.depth;
  require(u[4]<=buf.length && total<=buf.length-u[4],@"texture copy buffer too small");
  // One row per blit permits tightly packed RHI buffers without Metal row-pitch padding.
  if(!c.compute) { c.compute=[c.buffer computeCommandEncoder]; require(c.compute!=nil,@"cannot create compute encoder"); [c.compute waitForFence:b.fence beforeEncoderStages:kComputeStages]; } id<MTL4ComputeCommandEncoder> e=c.compute;
  for(NSUInteger z=0;z<size.depth;z++) for(NSUInteger y=0;y<size.height;y++) {
   NSUInteger offset=u[4]+z*img+y*row; MTLOrigin origin=MTLOriginMake(0,y,z); MTLSize line=MTLSizeMake(size.width,1,1);
   if(op==MBUpload) [e copyFromBuffer:buf sourceOffset:offset sourceBytesPerRow:row sourceBytesPerImage:row sourceSize:line toTexture:t destinationSlice:u[2] destinationLevel:u[1] destinationOrigin:origin];
   else [e copyFromTexture:t sourceSlice:u[2] sourceLevel:u[1] sourceOrigin:origin sourceSize:line toBuffer:buf destinationOffset:offset destinationBytesPerRow:row destinationBytesPerImage:row];
  }
  return 0;
 }
 case MBSurface: {
  require([NSThread isMainThread],@"CreateMetalSurface must run on the main thread"); NSWindow *w=(NSWindow*)p[0]; require(w!=nil,@"nil NSWindow");
  NSView *v=w.contentView; if(![v.layer isKindOfClass:CAMetalLayer.class]) {v.wantsLayer=YES; v.layer=[CAMetalLayer layer];}
  CAMetalLayer *layer=(CAMetalLayer*)v.layer; CGFloat scale=w.backingScaleFactor; CGSize points=v.bounds.size;
  layer.contentsScale=scale; layer.drawableSize=CGSizeMake(points.width*scale,points.height*scale); return (uintptr_t)layer;
 }
 case MBSwapchain: {
  CAMetalLayer *layer=(CAMetalLayer*)(uintptr_t)u[0]; require([layer isKindOfClass:CAMetalLayer.class],@"surface must be a CAMetalLayer");
  layer.device=b.device; layer.pixelFormat=MTLPixelFormatBGRA8Unorm; layer.framebufferOnly=NO; layer.drawableSize=CGSizeMake(u[1],u[2]); return add(b,layer,6);
 }
 case MBResize: {
  MBResource *r=resource(b,u[0],6); CAMetalLayer *l=r.object;
  if(u[3]) {u[1]=l.drawableSize.width;u[2]=l.drawableSize.height;} else {require(!r.auxiliary,@"cannot resize with an acquired drawable"); l.drawableSize=CGSizeMake(u[1],u[2]);} return 0;
 }
 case MBAcquire: {
  MBResource *r=resource(b,u[0],6); require(!r.auxiliary,@"drawable already acquired"); id<CAMetalDrawable> d=[(CAMetalLayer*)r.object nextDrawable]; require(d!=nil,@"no drawable available (window hidden or zero size)"); r.auxiliary=d;
  // A drawable is used only as a render attachment; it is never referenced through
  // a bindless argument table. Keeping it out of the residency set means its
  // acquire/release cycle costs nothing beyond this call.
  uint64_t h=addWithResidency(b,d.texture,2,NO); r.slot=h; return h;
 }
 case MBPresent: {MBResource *r=resource(b,u[0],6); require(r.auxiliary!=nil,@"no acquired drawable");
  resource(b,r.slot,2); uint64_t h=submit(b,u[1],r.auxiliary);
  [b.resources removeObjectForKey:@(r.slot)]; r.slot=0; r.auxiliary=nil;
  // gamekit reuses mapped frame data and reads counters immediately after Present.
  // Match the synchronous presentation contract of the Vulkan backend.
  NSNumber *v=b.fences[@(h)]; completed(b,v.unsignedLongLongValue);
  [b.fences removeObjectForKey:@(h)]; return h;}
 case MBPool: {
  if(!u[0]) return 0;
  MTL4CounterHeapDescriptor *d=[MTL4CounterHeapDescriptor new];
  d.type=MTL4CounterHeapTypeTimestamp; d.count=u[0];
  NSError *error=nil; id<MTL4CounterHeap> heap=[b.device newCounterHeapWithDescriptor:d error:&error]; [d release];
  if(!heap) return 0; // unsupported device: degrade the same way this bridge always has.
  uint64_t h=add(b,heap,7); MBResource *r=resource(b,h,7); r.auxiliary=[NSMutableIndexSet indexSet]; r.timestampOverrides=[NSMutableDictionary dictionary]; MTLTimestamp cpu,gpu; [b.device sampleTimestamps:&cpu gpuTimestamp:&gpu]; r.cpuStart=cpu; r.gpuStart=gpu; return h;
 }
 case MBResetTimes: {if(!u[0]) return 0; outside(b,c); MBResource *r=resource(b,u[0],7); id<MTL4CounterHeap> heap=r.object; require(u[1]<=heap.count,@"query reset out of bounds"); [heap invalidateCounterRange:NSMakeRange(0,u[1])]; [(NSMutableIndexSet*)r.auxiliary removeIndexesInRange:NSMakeRange(0,u[1])]; for(NSUInteger i=0;i<u[1];i++) [r.timestampOverrides removeObjectForKey:@(i)]; return 0;}
 case MBTimestamp: {
  if(!u[0]) return 0; MBResource *r=resource(b,u[0],7); id<MTL4CounterHeap> heap=r.object; require(u[1]<heap.count,@"query index out of bounds");
  if(!c.pendingQueries) { c.pendingQueries=[NSMutableArray array]; c.startQueries=[NSMutableArray array]; c.endQueries=[NSMutableArray array]; }
  MBQuery *q=[MBQuery new]; q.pool=r; q.index=u[1]; q.stage=u[2]; [c.pendingQueries addObject:q]; [q release]; [(NSMutableIndexSet*)r.auxiliary addIndex:u[1]]; return 0;
 }
 case MBReadTimes: {
  MBResource *r=resource(b,u[0],7); id<MTL4CounterHeap> heap=r.object; require(u[1]<=heap.count,@"query read out of bounds");
  NSData *data=[heap resolveCounterRange:NSMakeRange(0,u[1])]; require(data.length>=u[1]*sizeof(MTL4TimestampHeapEntry),@"timestamp resolve failed");
  MTLTimestamp cpu,gpu; [b.device sampleTimestamps:&cpu gpuTimestamp:&gpu];
  mach_timebase_info_data_t timebase; mach_timebase_info(&timebase);
  long double cpuScale=(long double)timebase.numer/timebase.denom;
  require(gpu>r.gpuStart,@"GPU timestamp calibration unavailable");
  long double period=(cpu-r.cpuStart)*cpuScale/(gpu-r.gpuStart);
  const MTL4TimestampHeapEntry *values=data.bytes; uint64_t *out=(uint64_t*)p[0];
  for(NSUInteger i=0;i<u[1];i++) {
   NSNumber *override=r.timestampOverrides[@(i)];
   out[i]=override ? override.unsignedLongLongValue : ([(NSIndexSet*)r.auxiliary containsIndex:i] && values[i].timestamp ?
    (uint64_t)((long double)r.cpuStart*cpuScale+((long double)values[i].timestamp-r.gpuStart)*period):0);
  }
  return 0;
 }
 }
 require(NO,@"unknown bridge operation");
 } @catch(NSException *e) {
  snprintf(mbErrorText,sizeof(mbErrorText),"%s",e.reason.UTF8String); a->error=1;
  // Close any encoder the failed op left open. Releasing one without endEncoding
  // aborts the process, which would bury the message above under an unrelated
  // assertion by the time the caller's panic unwinds.
  if(c) { if(c.render) { [c.render endEncoding]; c.render=nil; } endCompute(b,c); }
 }
 return 0;
 }
}
