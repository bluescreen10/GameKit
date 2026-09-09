//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include "bridge.h"
#include <mach/mach_time.h>

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
@property(retain) id<MTLCommandBuffer> buffer;
@property(retain) id<MTLRenderCommandEncoder> render;
@property(retain) id<MTLComputeCommandEncoder> compute;
@property(retain) id<MTLBlitCommandEncoder> blit;
@property(retain) MTLRenderPassDescriptor *pass;
@property(retain) MBResource *pipeline;
@property(retain) id<MTLBuffer> textureTable;
@property(retain) id<MTLBuffer> samplerTable;
@property uint64_t root;
@property MTLViewport viewport;
@property MTLScissorRect scissor;
@property BOOL hasViewport;
@property BOOL hasScissor;
@property BOOL readOnlyDepth;
@property(retain) NSMutableArray *pendingQueries;
@property(retain) NSMutableArray *startQueries;
@property(retain) NSMutableArray *endQueries;

@end
@implementation MBCommand
- (void)dealloc { [_buffer release]; [_render release]; [_compute release]; [_blit release]; [_pass release]; [_pipeline release]; [_textureTable release]; [_samplerTable release]; [_pendingQueries release]; [_startQueries release]; [_endQueries release]; [super dealloc]; }
@end
@interface MBDevice : NSObject
@property(retain) id<MTLDevice> device;
@property(retain) id<MTLCommandQueue> queue;
@property(retain) NSMutableDictionary *resources;
@property(retain) NSMutableArray *residentResources;
@property(retain) NSMutableData *residentPointers;
@property(retain) NSMutableDictionary *commands;
@property(retain) NSMutableDictionary *fences;
@property(retain) id<MTLBuffer> textures;
@property(retain) id<MTLBuffer> samplers;
@property(retain) id<MTLBuffer> textureSnapshot;
@property(retain) id<MTLBuffer> samplerSnapshot;
@property(retain) id<MTLBuffer> timestampScratch;
@property(retain) NSMutableIndexSet *textureSlots;
@property(retain) NSMutableIndexSet *samplerSlots;
// An MTLResidencySet (macOS 15+) attached to the queue keeps every allocation
// resident for the queue's lifetime, so encoders need no useResources: call at
// all. It is held as id to keep the declaration free of availability
// annotations; residentResources is the pre-15 fallback and stays empty when a
// set is in use.
@property(retain) id residencySet;
@property BOOL residencyDirty;
@property uint64_t next;
@end
@implementation MBDevice
- (void)dealloc {
 [_fences release]; [_commands release]; [_resources release]; [_residentResources release]; [_residentPointers release]; [_textures release]; [_samplers release]; [_textureSnapshot release]; [_samplerSnapshot release];
 [_timestampScratch release]; [_textureSlots release]; [_samplerSlots release]; [_residencySet release]; [_queue release]; [_device release]; [super dealloc];
}
@end
static void require(BOOL condition, NSString *message) {
 if (!condition) [NSException raise:@"MetalRHI" format:@"%@",message];
}
static NSString *str(const void *s) {return s ? [NSString stringWithUTF8String:s] : @"";}
static MBResource *resource(MBDevice *b,uint64_t h,NSUInteger kind) {
 MBResource *r=b.resources[@(h)]; require(r && (!kind || r.kind==kind),@"invalid resource handle or kind"); return r;
}
static uint64_t addWithResidency(MBDevice *b,id obj,NSUInteger kind,BOOL resident) {
 require(obj!=nil,@"Metal resource creation failed"); MBResource *r=[MBResource new]; r.object=obj; r.kind=kind;
 uint64_t h=++b.next; b.resources[@(h)]=r; [r release];
 if(resident && (kind==1||kind==2)) {
  if(b.residencySet) { if (@available(macOS 15.0,*)) [(id<MTLResidencySet>)b.residencySet addAllocation:(id<MTLAllocation>)obj]; }
  else [b.residentResources addObject:obj];
  b.residencyDirty=YES;
 }
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
 if(sampler) b.samplerSnapshot=nil; else b.textureSnapshot=nil;
 return i;
}
static void endCompute(MBCommand *c) { if(c.compute) { [c.compute endEncoding]; c.compute=nil; } }
static void endBlit(MBCommand *c) { if(c.blit) { [c.blit endEncoding]; c.blit=nil; } }
static void outside(MBCommand *c) {require(!c.render,@"operation must be outside a render pass"); endCompute(c);}
static void residency(MBDevice *b,MBCommand *c) {
 // Reuse immutable descriptor snapshots until a slot changes. Command encoders retain
 // their snapshot, so rebuilding the device cache cannot mutate in-flight tables.
 if(!b.textureSnapshot) b.textureSnapshot=[[b.device newBufferWithBytes:b.textures.contents length:b.textures.length options:MTLResourceStorageModeShared] autorelease];
 if(!b.samplerSnapshot) b.samplerSnapshot=[[b.device newBufferWithBytes:b.samplers.contents length:b.samplers.length options:MTLResourceStorageModeShared] autorelease];
 c.textureTable=b.textureSnapshot;
 c.samplerTable=b.samplerSnapshot;
 require(c.textureTable && c.samplerTable,@"cannot snapshot argument tables");
 // The ABI allows arbitrary pointer chains, so every resource has to be declared.
 // The residency set does that once for the whole queue: re-commit only after the
 // set actually changed, and the encoders themselves declare nothing.
 if(b.residencySet) {
  if(b.residencyDirty) {
   if (@available(macOS 15.0,*)) { [(id<MTLResidencySet>)b.residencySet commit]; [(id<MTLResidencySet>)b.residencySet requestResidency]; }
   b.residencyDirty=NO;
  }
  return;
 }
 // Fallback for macOS 13 and 14: declare everything per encoder, batching the
 // declaration to avoid one Objective-C call per object and encoder.
 if(b.residencyDirty) {
  b.residentPointers.length=b.residentResources.count*sizeof(id<MTLResource>);
  if(b.residentResources.count) [b.residentResources getObjects:b.residentPointers.mutableBytes range:NSMakeRange(0,b.residentResources.count)];
  b.residencyDirty=NO;
 }
 NSUInteger count=b.residentResources.count;
 const id<MTLResource> *resources=b.residentPointers.bytes;
 if(count && c.render) [c.render useResources:resources count:count usage:MTLResourceUsageRead|MTLResourceUsageWrite stages:MTLRenderStageVertex|MTLRenderStageFragment];
 if(count && c.compute) [c.compute useResources:resources count:count usage:MTLResourceUsageRead|MTLResourceUsageWrite];
}
// Render-pass splits implement coarse barriers/timestamps on tile-based GPUs.
// Preserve attachment contents and dynamic state when resuming the logical pass.
static void pauseRender(MBCommand *c) {
 for(NSUInteger i=0;i<8;i++) if(c.pass.colorAttachments[i].texture) [c.render setColorStoreAction:MTLStoreActionStore atIndex:i];
 if(c.pass.depthAttachment.texture) [c.render setDepthStoreAction:MTLStoreActionStore];
 [c.render endEncoding]; c.render=nil;
}
static void resumeRender(MBDevice *b,MBCommand *c,BOOL load) {
 MTLRenderPassDescriptor *d=[[c.pass copy] autorelease];
 for(NSUInteger i=0;i<8;i++) if(d.colorAttachments[i].texture) { if(load) d.colorAttachments[i].loadAction=MTLLoadActionLoad; d.colorAttachments[i].storeAction=MTLStoreActionUnknown; }
 if(d.depthAttachment.texture) { if(load) d.depthAttachment.loadAction=MTLLoadActionLoad; d.depthAttachment.storeAction=MTLStoreActionUnknown; }
 c.render=[c.buffer renderCommandEncoderWithDescriptor:d]; require(c.render!=nil,@"cannot resume render pass"); residency(b,c);
 [c.render setVertexBuffer:c.textureTable offset:0 atIndex:1]; [c.render setFragmentBuffer:c.textureTable offset:0 atIndex:1];
 [c.render setVertexBuffer:c.textureTable offset:0 atIndex:3]; [c.render setFragmentBuffer:c.textureTable offset:0 atIndex:3];
 [c.render setVertexBuffer:c.samplerTable offset:0 atIndex:2]; [c.render setFragmentBuffer:c.samplerTable offset:0 atIndex:2];
 if(c.hasViewport) [c.render setViewport:c.viewport]; if(c.hasScissor) [c.render setScissorRect:c.scissor];
}
static void sampleQuery(MBDevice *b,MBCommand *c,MBQuery *q) {
 BOOL resume=c.render!=nil;
 if(resume) pauseRender(c); else outside(c);
 endBlit(c);
 id<MTLCounterSampleBuffer> pool=q.pool.object;
 MTLBlitPassDescriptor *d=[MTLBlitPassDescriptor blitPassDescriptor];
 d.sampleBufferAttachments[0].sampleBuffer=pool;
 d.sampleBufferAttachments[0].startOfEncoderSampleIndex=q.index;
 d.sampleBufferAttachments[0].endOfEncoderSampleIndex=MTLCounterDontSample;
 id<MTLBlitCommandEncoder> e=[c.buffer blitCommandEncoderWithDescriptor:d];
 [e fillBuffer:b.timestampScratch range:NSMakeRange(0,4) value:0];
 [e endEncoding];
 if(resume) resumeRender(b,c,YES);
}
// Stage-boundary-only Apple GPUs cannot sample an arbitrary point in an open
// encoder. Defer a query until immediately before the next real operation. A
// leading StageNone query and queries with no following work are resolved from
// the command buffer's GPU boundaries when it completes.
static void prepareWork(MBDevice *b,MBCommand *c) {
 if(!c.pendingQueries.count) return;
 for(MBQuery *q in c.pendingQueries) {
  if(q.stage==0) [c.startQueries addObject:q];
  else sampleQuery(b,c,q);
 }
 [c.pendingQueries removeAllObjects];
}
// bindings pushes the draw/dispatch data inline with setBytes, Metal's equivalent of
// push constants. The data belongs to the call rather than to the encoder, so it is
// passed in rather than read from sticky state — a draw that supplies none pushes
// nothing rather than silently inheriting the previous draw's.
static void bindings(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 if(!data || size==0) return;
 require(size<=4096,@"draw/dispatch data exceeds Metal's 4KB setBytes limit");
 if(c.render) {
  [c.render setVertexBytes:data length:size atIndex:0]; [c.render setFragmentBytes:data length:size atIndex:0];
 }
 if(c.compute) [c.compute setBytes:data length:size atIndex:0];
}
static void compute(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 require(!c.render && c.pipeline.kind==5,@"dispatch requires a compute pipeline outside a render pass");
 if(!c.compute) {
  endBlit(c); c.compute=[c.buffer computeCommandEncoder]; require(c.compute!=nil,@"cannot create compute encoder"); residency(b,c);
  [c.compute setBuffer:c.textureTable offset:0 atIndex:1]; [c.compute setBuffer:c.samplerTable offset:0 atIndex:2]; [c.compute setBuffer:c.textureTable offset:0 atIndex:3];
 }
 [c.compute setComputePipelineState:c.pipeline.object]; bindings(b,c,data,size);
}
static void draw(MBDevice *b,MBCommand *c,const void *data,uint32_t size) {
 require(c.render && c.pipeline.kind==4,@"draw requires a graphics pipeline and render pass");
 require(!c.readOnlyDepth || !c.pipeline.depthWrite,@"depth-writing pipeline in read-only depth pass");
 [c.render setRenderPipelineState:c.pipeline.object]; [c.render setDepthStencilState:c.pipeline.auxiliary];
 [c.render setCullMode:c.pipeline.cull]; [c.render setFrontFacingWinding:c.pipeline.clockwise?MTLWindingClockwise:MTLWindingCounterClockwise]; bindings(b,c,data,size);
}
static id<MTLFunction> function(MBDevice *b,const void *data,NSUInteger size,const void *entry) {
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
 id<MTLFunction> f=[library newFunctionWithName:str(entry)]; [library release];
 require(f!=nil,[NSString stringWithFormat:@"shader entry %@ not found",str(entry)]); return [f autorelease];
}
static void completed(id<MTLCommandBuffer> cb) { [cb waitUntilCompleted]; require(cb.status!=MTLCommandBufferStatusError,cb.error.localizedDescription ?: @"GPU command failed"); }
static void completedSubmission(MBDevice *b,id<MTLCommandBuffer> cb) {
 completed(cb);
}
static void idle(MBDevice *b) { for(id<MTLCommandBuffer> cb in b.fences.allValues) completedSubmission(b,cb); [b.fences removeAllObjects]; }
static uint64_t submit(MBDevice *b,uint64_t h,id<CAMetalDrawable> drawable) {
 // An allocation made after the last encoder was created never reached residency(),
 // so catch it here: the GPU must not run against an uncommitted set.
 if(b.residencySet && b.residencyDirty) {
  if (@available(macOS 15.0,*)) { [(id<MTLResidencySet>)b.residencySet commit]; [(id<MTLResidencySet>)b.residencySet requestResidency]; }
  b.residencyDirty=NO;
 }
 MBCommand *c=b.commands[@(h)]; require(c!=nil,@"invalid command buffer"); outside(c);
 endBlit(c);
 if(c.pendingQueries.count) { if(!c.endQueries) c.endQueries=[NSMutableArray array]; [c.endQueries addObjectsFromArray:c.pendingQueries]; [c.pendingQueries removeAllObjects]; }
 if(drawable) [c.buffer presentDrawable:drawable];
 // Retain indirectly referenced objects through GPU completion as well.
 NSArray *objects=[b.resources.allValues copy];
 NSArray *starts=[c.startQueries copy],*ends=[c.endQueries copy];
 if(starts.count||ends.count) [c.buffer addCompletedHandler:^(id<MTLCommandBuffer> cb) {
  // GPUStartTime/GPUEndTime use Core Animation's host clock. Translate that
  // clock to mach absolute nanoseconds so boundary and hardware-counter samples
  // returned from one query pool remain directly comparable.
  mach_timebase_info_data_t timebase; mach_timebase_info(&timebase);
  long double hostNow=(long double)mach_absolute_time()*timebase.numer/timebase.denom;
  long double hostOrigin=hostNow-(long double)CACurrentMediaTime()*1e9L;
  uint64_t start=cb.GPUStartTime>0 ? (uint64_t)(hostOrigin+(long double)cb.GPUStartTime*1e9L) : 0;
  uint64_t end=cb.GPUEndTime>0 ? (uint64_t)(hostOrigin+(long double)cb.GPUEndTime*1e9L) : 0;
  for(MBQuery *q in starts) q.pool.timestampOverrides[@(q.index)]=@(start);
  for(MBQuery *q in ends) q.pool.timestampOverrides[@(q.index)]=@(end);
  (void)objects;
 }];
 [objects release]; [starts release]; [ends release];
 [c.buffer commit]; b.fences[@(h)]=c.buffer; [b.commands removeObjectForKey:@(h)]; return h;
}
void *mbCreate(void) { return [MBDevice new]; }
void *mbPointer(uint64_t v) { return (void*)(uintptr_t)v; }
// The message for the most recent failed call. Only MBArgs.error says whether it
// is current, so it is never cleared on the success path.
static char mbErrorText[2048];
const char *mbError(void) { return mbErrorText; }
uint64_t mbCall(void *backend,int op,MBArgs *a) {
 @autoreleasepool { @try {
 MBDevice *b=backend; uint64_t *u=a->u; double *f=a->f; const void **p=a->p;
 MBCommand *c=u[30] ? b.commands[@(u[30])] : nil;
 if(u[30]) require(c!=nil,@"invalid command buffer");
 switch(op) {
 case MBInit: {
  if (@available(macOS 13.0,*)) {} else { require(NO,@"macOS 13 or later is required"); }
  b.device=[MTLCreateSystemDefaultDevice() autorelease]; require(b.device!=nil,@"no Metal device");
  require(b.device.argumentBuffersSupport==MTLArgumentBuffersTier2 && b.device.hasUnifiedMemory,@"requires a unified-memory GPU with Tier 2 argument buffers");
  b.queue=[[b.device newCommandQueue] autorelease]; require(b.queue!=nil,@"cannot create command queue");
  if (@available(macOS 15.0,*)) {
   MTLResidencySetDescriptor *rd=[MTLResidencySetDescriptor new]; rd.label=@"gamekit.residency";
   NSError *rerr=nil; id<MTLResidencySet> set=[b.device newResidencySetWithDescriptor:rd error:&rerr]; [rd release];
   // A failure here is not fatal: residency() falls back to useResources:.
   if(set) { b.residencySet=set; [set release]; [b.queue addResidencySet:set]; }
  }
  b.resources=[NSMutableDictionary dictionary]; b.residentResources=[NSMutableArray array]; b.residentPointers=[NSMutableData data]; b.residencyDirty=YES;
  b.commands=[NSMutableDictionary dictionary]; b.fences=[NSMutableDictionary dictionary];
  b.textures=[[b.device newBufferWithLength:65536*8 options:MTLResourceStorageModeShared] autorelease];
  b.samplers=[[b.device newBufferWithLength:2048*8 options:MTLResourceStorageModeShared] autorelease];
  require(b.textures && b.samplers,@"cannot allocate argument tables");
  b.textureSlots=[NSMutableIndexSet indexSetWithIndexesInRange:NSMakeRange(1,65535)];
  b.timestampScratch=[[b.device newBufferWithLength:4 options:MTLResourceStorageModeShared] autorelease];
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
  if(r.slot) { BOOL s=r.kind==3; ((uint64_t*)[(s?b.samplers:b.textures) contents])[r.slot]=0; [(s?b.samplerSlots:b.textureSlots) addIndex:r.slot]; if(s) b.samplerSnapshot=nil; else b.textureSnapshot=nil; }
  if(r.kind==1||r.kind==2) {
   if(b.residencySet) { if (@available(macOS 15.0,*)) [(id<MTLResidencySet>)b.residencySet removeAllocation:(id<MTLAllocation>)r.object]; }
   else [b.residentResources removeObjectIdenticalTo:r.object];
   b.residencyDirty=YES;
  }
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
  id<MTLFunction> fn=function(b,p[0],u[0],p[1]); NSError *error=nil;
  id<MTLComputePipelineState> ps=[b.device newComputePipelineStateWithFunction:fn error:&error]; require(ps!=nil,error.localizedDescription);
  MTLSize group=MTLSizeMake(MAX(u[1],1),MAX(u[2],1),MAX(u[3],1));
  require(group.width*group.height*group.depth<=ps.maxTotalThreadsPerThreadgroup,@"workgroup too large");
  uint64_t h=add(b,ps,5); [ps release]; resource(b,h,5).group=group; return h;
 }
 case MBPipeline: {
  MTLRenderPipelineDescriptor *d=[MTLRenderPipelineDescriptor new];
  d.vertexFunction=function(b,p[0],u[0],p[2]); if(u[1]) d.fragmentFunction=function(b,p[1],u[1],p[3]);
  d.rasterSampleCount=MAX(u[4],1); d.depthAttachmentPixelFormat=format(u[3]); if(u[3]==13) d.stencilAttachmentPixelFormat=format(u[3]);
  static const MTLBlendFactor factors[]={MTLBlendFactorZero,MTLBlendFactorOne,MTLBlendFactorSourceAlpha,MTLBlendFactorOneMinusSourceAlpha,MTLBlendFactorDestinationAlpha,MTLBlendFactorOneMinusDestinationAlpha};
  for(NSUInteger i=0;i<u[10];i++) {
   MTLRenderPipelineColorAttachmentDescriptor *t=d.colorAttachments[i]; t.pixelFormat=format(u[11+i]); uint64_t v=u[19+i];
   t.writeMask=v?((v>>1)&15):MTLColorWriteMaskAll;
   // RHI mask uses R at bit 0; Metal uses R at bit 3.
   NSUInteger mask=t.writeMask; t.writeMask=((mask&1)<<3)|((mask&2)<<1)|((mask&4)>>1)|((mask&8)>>3);
   t.blendingEnabled=v&1;
   if(v&1) {NSUInteger sc=(v>>5)&15,dc=(v>>9)&15,sa=(v>>17)&15,da=(v>>21)&15; require(sc<6&&dc<6&&sa<6&&da<6,@"invalid blend factor"); t.sourceRGBBlendFactor=factors[sc]; t.destinationRGBBlendFactor=factors[dc]; t.rgbBlendOperation=(v>>13)&15; t.sourceAlphaBlendFactor=factors[sa]; t.destinationAlphaBlendFactor=factors[da]; t.alphaBlendOperation=(v>>25)&15;}
  }
  NSError *error=nil; id<MTLRenderPipelineState> ps=[b.device newRenderPipelineStateWithDescriptor:d error:&error]; [d release]; require(ps!=nil,error.localizedDescription);
  uint64_t h=add(b,ps,4); [ps release]; MBResource *r=resource(b,h,4);
  MTLDepthStencilDescriptor *depth=[MTLDepthStencilDescriptor new]; depth.depthCompareFunction=u[7]?u[9]:MTLCompareFunctionAlways; depth.depthWriteEnabled=u[8];
  r.auxiliary=[[b.device newDepthStencilStateWithDescriptor:depth] autorelease]; [depth release];
  const MTLPrimitiveType topologies[]={MTLPrimitiveTypeTriangle,MTLPrimitiveTypeTriangleStrip,MTLPrimitiveTypeLine,MTLPrimitiveTypePoint}; require(u[2]<4&&u[5]<3,@"invalid topology or cull mode");
  r.topology=topologies[u[2]]; r.cull=u[5]==1?MTLCullModeBack:u[5]==2?MTLCullModeFront:MTLCullModeNone; r.clockwise=u[6]; r.depthWrite=u[8]; return h;
 }
 case MBBegin: { MBCommand *v=[MBCommand new]; v.buffer=[b.queue commandBuffer]; require(v.buffer!=nil,@"cannot create command buffer"); uint64_t h=++b.next; b.commands[@(h)]=v; [v release]; return h; }
 case MBSubmit: return submit(b,u[0],nil);
 case MBWait: {id<MTLCommandBuffer> cb=b.fences[@(u[0])]; if(cb) {completedSubmission(b,cb); [b.fences removeObjectForKey:@(u[0])];} return 0;}
 case MBIdle: idle(b); return 0;
 case MBRenderBegin: {
  require(!c.render,@"nested render pass"); endCompute(c); endBlit(c); MBRenderDesc *v=(MBRenderDesc*)p[0]; require(v && v->colorCount<=8,@"invalid render pass");
  c.pass=[MTLRenderPassDescriptor renderPassDescriptor];
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
  [c.render endEncoding]; c.render=nil; c.pass=nil; c.hasViewport=NO; c.hasScissor=NO; c.readOnlyDepth=NO; return 0;
 case MBSetPipeline: c.pipeline=resource(b,u[0],0); require(c.pipeline.kind==4||c.pipeline.kind==5,@"invalid pipeline"); return 0;
 case MBViewport: require(c.render!=nil,@"viewport outside render pass"); c.viewport=(MTLViewport){f[0],f[1],f[2],f[3],f[4],f[5]}; c.hasViewport=YES; [c.render setViewport:c.viewport]; return 0;
 case MBScissor: require(c.render!=nil,@"scissor outside render pass"); c.scissor=(MTLScissorRect){u[0],u[1],u[2],u[3]}; c.hasScissor=YES; [c.render setScissorRect:c.scissor]; return 0;
 case MBDraw: prepareWork(b,c); draw(b,c,a->p[0],(uint32_t)u[29]); [c.render drawPrimitives:c.pipeline.topology vertexStart:u[2] vertexCount:u[0] instanceCount:u[1] baseInstance:u[3]]; return 0;
 case MBIndexed: {prepareWork(b,c); draw(b,c,a->p[0],(uint32_t)u[29]);
  // Metal takes a BYTE offset here, unlike Vulkan which takes an element index and
  // derives the stride from the bound index type — so firstIndex must be scaled by
  // the index width rather than a hardcoded 4.
  MTLIndexType it = u[6]==2 ? MTLIndexTypeUInt16 : MTLIndexTypeUInt32;
  [c.render drawIndexedPrimitives:c.pipeline.topology indexCount:u[1] indexType:it indexBuffer:resource(b,u[0],1).object indexBufferOffset:u[3]*u[6] instanceCount:u[2] baseVertex:(NSInteger)u[4] baseInstance:u[5]]; return 0;}
 case MBIndirect: {
  prepareWork(b,c); draw(b,c,a->p[0],(uint32_t)u[29]); id<MTLBuffer> buf=resource(b,u[1],1).object; require(u[4]>=20 && !(u[4]%4) && !(u[2]%4),@"invalid indirect stride/offset"); require(!u[3] || u[2]+(u[3]-1)*u[4]+20<=buf.length,@"indirect draw out of bounds");
  MTLIndexType it = u[5]==2 ? MTLIndexTypeUInt16 : MTLIndexTypeUInt32;
  // Metal has no multi-draw-indirect, so one draw per command is the only option.
  // The index buffer is the same for all of them: resolving the handle inside the
  // loop would box an NSNumber and hash the resource table once per command.
  id<MTLBuffer> ib=resource(b,u[0],1).object;
  for(NSUInteger i=0;i<u[3];i++) [c.render drawIndexedPrimitives:c.pipeline.topology indexType:it indexBuffer:ib indexBufferOffset:0 indirectBuffer:buf indirectBufferOffset:u[2]+i*u[4]]; return 0;
 }
 case MBDispatch: prepareWork(b,c); compute(b,c,a->p[0],(uint32_t)u[29]); [c.compute dispatchThreadgroups:MTLSizeMake(u[0],u[1],u[2]) threadsPerThreadgroup:c.pipeline.group]; return 0;
 case MBDispatchIndirect: {prepareWork(b,c); compute(b,c,a->p[0],(uint32_t)u[29]); id<MTLBuffer> buf=resource(b,u[0],1).object; require(u[1]%4==0 && u[1]+12<=buf.length,@"indirect dispatch out of bounds"); [c.compute dispatchThreadgroupsWithIndirectBuffer:buf indirectBufferOffset:u[1] threadsPerThreadgroup:c.pipeline.group]; return 0;}
 case MBBarrier:
  if(c.render) {pauseRender(c); resumeRender(b,c,YES);}
  else {endCompute(c); endBlit(c);} return 0;
 case MBCopyBuffer: {
  outside(c); prepareWork(b,c); id<MTLBuffer> dst=resource(b,u[0],1).object,src=resource(b,u[1],1).object;
  require(u[2]<=dst.length && u[4]<=dst.length-u[2] && u[3]<=src.length && u[4]<=src.length-u[3],@"buffer copy out of bounds");
  if(!c.blit) c.blit=[c.buffer blitCommandEncoder]; require(c.blit!=nil,@"cannot create blit encoder");
  [c.blit copyFromBuffer:src sourceOffset:u[3] toBuffer:dst destinationOffset:u[2] size:u[4]]; return 0;
 }
 case MBUpload: case MBReadback: {
  outside(c); prepareWork(b,c); id<MTLTexture> t=resource(b,u[0],2).object; id<MTLBuffer> buf=resource(b,u[3],1).object;
  require(u[1]<t.mipmapLevelCount && t.sampleCount==1,@"invalid copy mip or multisampled texture");
  NSUInteger layers=t.arrayLength*((t.textureType==MTLTextureTypeCube||t.textureType==MTLTextureTypeCubeArray)?6:1); require(u[2]<layers,@"copy layer out of bounds");
  NSUInteger bytes=0; switch(t.pixelFormat) {case MTLPixelFormatR8Unorm:bytes=1;break; case MTLPixelFormatRG8Unorm:bytes=2;break;case MTLPixelFormatRGBA16Float:bytes=8;break;case MTLPixelFormatRGBA32Float:bytes=16;break;default:bytes=4;}
  MTLSize size=MTLSizeMake(MAX(t.width>>u[1],1),MAX(t.height>>u[1],1),MAX(t.depth>>u[1],1)); NSUInteger row=size.width*bytes,img=row*size.height,total=img*size.depth;
  require(u[4]<=buf.length && total<=buf.length-u[4],@"texture copy buffer too small");
  // One row per blit permits tightly packed RHI buffers without Metal row-pitch padding.
  if(!c.blit) c.blit=[c.buffer blitCommandEncoder]; require(c.blit!=nil,@"cannot create blit encoder"); id<MTLBlitCommandEncoder> e=c.blit;
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
  // a bindless argument table. Keeping it out of residentResources prevents its
  // acquire/release cycle from rebuilding the entire residency snapshot each frame.
  uint64_t h=addWithResidency(b,d.texture,2,NO); r.slot=h; return h;
 }
 case MBPresent: {MBResource *r=resource(b,u[0],6); require(r.auxiliary!=nil,@"no acquired drawable");
  resource(b,r.slot,2); uint64_t h=submit(b,u[1],r.auxiliary);
  [b.resources removeObjectForKey:@(r.slot)]; r.slot=0; r.auxiliary=nil;
  // gamekit reuses mapped frame data and reads counters immediately after Present.
  // Match the synchronous presentation contract of the Vulkan backend.
  id<MTLCommandBuffer> finished=b.fences[@(h)]; completedSubmission(b,finished);
  [b.fences removeObjectForKey:@(h)]; return h;}
 case MBPool: {
  // Stage-boundary samples work on Apple GPUs; unsupported devices return an invalid pool.
  if(![b.device supportsCounterSampling:MTLCounterSamplingPointAtStageBoundary] || !u[0]) return 0;
  id<MTLCounterSet> set=nil; for(id<MTLCounterSet> s in b.device.counterSets) if([s.name isEqualToString:MTLCommonCounterSetTimestamp]) set=s;
  if(!set) return 0; MTLCounterSampleBufferDescriptor *d=[MTLCounterSampleBufferDescriptor new]; d.counterSet=set; d.sampleCount=u[0]; d.storageMode=MTLStorageModeShared;
  NSError *error=nil; id<MTLCounterSampleBuffer> pool=[b.device newCounterSampleBufferWithDescriptor:d error:&error]; [d release]; if(!pool) return 0;
  uint64_t h=add(b,pool,7); [pool release]; MBResource *r=resource(b,h,7); r.auxiliary=[NSMutableIndexSet indexSet]; r.timestampOverrides=[NSMutableDictionary dictionary]; MTLTimestamp cpu,gpu; [b.device sampleTimestamps:&cpu gpuTimestamp:&gpu]; r.cpuStart=cpu; r.gpuStart=gpu; return h;
 }
 case MBResetTimes: {if(!u[0]) return 0; outside(c); MBResource *r=resource(b,u[0],7); require(u[1]<=[(id<MTLCounterSampleBuffer>)r.object sampleCount],@"query reset out of bounds"); [(NSMutableIndexSet*)r.auxiliary removeIndexesInRange:NSMakeRange(0,u[1])]; for(NSUInteger i=0;i<u[1];i++) [r.timestampOverrides removeObjectForKey:@(i)]; return 0;}
 case MBTimestamp: {
  if(!u[0]) return 0; MBResource *r=resource(b,u[0],7); id<MTLCounterSampleBuffer> pool=r.object; require(u[1]<pool.sampleCount,@"query index out of bounds");
  if(!c.pendingQueries) { c.pendingQueries=[NSMutableArray array]; c.startQueries=[NSMutableArray array]; c.endQueries=[NSMutableArray array]; }
  MBQuery *q=[MBQuery new]; q.pool=r; q.index=u[1]; q.stage=u[2]; [c.pendingQueries addObject:q]; [q release]; [(NSMutableIndexSet*)r.auxiliary addIndex:u[1]]; return 0;
 }
 case MBReadTimes: {
  MBResource *r=resource(b,u[0],7); id<MTLCounterSampleBuffer> pool=r.object; require(u[1]<=pool.sampleCount,@"query read out of bounds");
  NSData *data=[pool resolveCounterRange:NSMakeRange(0,u[1])]; require(data.length>=u[1]*sizeof(MTLCounterResultTimestamp),@"timestamp resolve failed");
  MTLTimestamp cpu,gpu; [b.device sampleTimestamps:&cpu gpuTimestamp:&gpu];
  mach_timebase_info_data_t timebase; mach_timebase_info(&timebase);
  long double cpuScale=(long double)timebase.numer/timebase.denom;
  require(gpu>r.gpuStart,@"GPU timestamp calibration unavailable");
  long double period=(cpu-r.cpuStart)*cpuScale/(gpu-r.gpuStart);
  const MTLCounterResultTimestamp *values=data.bytes; uint64_t *out=(uint64_t*)p[0];
  for(NSUInteger i=0;i<u[1];i++) {
   NSNumber *override=r.timestampOverrides[@(i)];
   out[i]=override ? override.unsignedLongLongValue : ([(NSIndexSet*)r.auxiliary containsIndex:i] && values[i].timestamp && values[i].timestamp!=MTLCounterErrorValue ?
    (uint64_t)((long double)r.cpuStart*cpuScale+((long double)values[i].timestamp-r.gpuStart)*period):0);
  }
  return 0;
 }
 }
 require(NO,@"unknown bridge operation");
 } @catch(NSException *e) {snprintf(mbErrorText,sizeof(mbErrorText),"%s",e.reason.UTF8String); a->error=1;}
 return 0;
 }
}
