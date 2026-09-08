#ifndef GAMEKIT_VULKAN_BRIDGE_H
#define GAMEKIT_VULKAN_BRIDGE_H

#include <stdint.h>
#include <stdlib.h>
#include <vulkan/vulkan.h>

#ifdef __cplusplus
extern "C" {
#endif

VkResult vkbCreateInstance(uint32_t nExt, const char* const* exts, VkInstance* out);

VkResult vkbPickPhysical(VkInstance inst, VkPhysicalDevice* out);

uint32_t vkbGraphicsQueueFamily(VkPhysicalDevice pd);

VkResult vkbCreateDevice(VkPhysicalDevice pd, uint32_t fam, VkDevice* outDev, VkQueue* outQueue);

void vkbDeviceName(VkPhysicalDevice pd, char* out);

float vkbMaxAnisotropy(VkPhysicalDevice pd);

uint32_t vkbApiVersion(VkPhysicalDevice pd);

VkResult vkbCreateBindlessHeap(VkDevice dev, VkPhysicalDevice phys,
                                      uint32_t wantSampled, uint32_t wantStorage, uint32_t wantSampler,
                                      VkDescriptorSetLayout* outLayout, VkDescriptorPool* outPool,
                                      VkDescriptorSet* outSet, VkPipelineLayout* outPipeLayout,
                                      uint32_t* capSampled, uint32_t* capStorage, uint32_t* capSampler);

VkResult vkbCreateSampler(VkDevice dev, VkFilter mag, VkFilter min, VkSamplerMipmapMode mip,
                                 VkSamplerAddressMode u, VkSamplerAddressMode v, VkSamplerAddressMode w,
                                 int compareEnable, VkCompareOp compareOp, float maxAniso, VkSampler* out);

void vkbWriteSampler(VkDevice dev, VkDescriptorSet set, uint32_t index, VkSampler s);

VkPipelineStageFlags2 vkbStageNone(void);

VkPipelineStageFlags2 vkbStageTop(void);

VkPipelineStageFlags2 vkbStageIndirect(void);

VkPipelineStageFlags2 vkbStageVertex(void);

VkPipelineStageFlags2 vkbStageFragment(void);

VkPipelineStageFlags2 vkbStageColor(void);

VkPipelineStageFlags2 vkbStageEarlyDepth(void);

VkPipelineStageFlags2 vkbStageLateDepth(void);

VkPipelineStageFlags2 vkbStageCompute(void);

VkPipelineStageFlags2 vkbStageTransfer(void);

VkPipelineStageFlags2 vkbStageCopy(void);

VkPipelineStageFlags2 vkbStageAll(void);

VkAccessFlags2 vkbAccessNone(void);

VkAccessFlags2 vkbAccessIndirectRead(void);

VkAccessFlags2 vkbAccessShaderRead(void);

VkAccessFlags2 vkbAccessShaderWrite(void);

VkAccessFlags2 vkbAccessColorWrite(void);

VkAccessFlags2 vkbAccessDepthRead(void);

VkAccessFlags2 vkbAccessDepthWrite(void);

VkAccessFlags2 vkbAccessTransferRead(void);

VkAccessFlags2 vkbAccessTransferWrite(void);

VkAccessFlags2 vkbAccessMemoryRead(void);

VkAccessFlags2 vkbAccessMemoryWrite(void);

VkResult vkbCreateCommandPool(VkDevice dev, uint32_t fam, VkCommandPool* out);

VkResult vkbAllocCmd(VkDevice dev, VkCommandPool pool, VkCommandBuffer* out);

VkResult vkbBeginCmd(VkCommandBuffer cb);

void vkbBindHeap(VkCommandBuffer cb, VkPipelineLayout layout, VkDescriptorSet set);

VkResult vkbSubmit(VkDevice dev, VkQueue q, VkCommandBuffer cb, VkFence* outFence);

void vkbImageBarrier(VkCommandBuffer cb, VkImage img, VkImageAspectFlags aspect,
                            VkImageLayout oldL, VkImageLayout newL,
                            VkPipelineStageFlags2 srcStage, VkAccessFlags2 srcAccess,
                            VkPipelineStageFlags2 dstStage, VkAccessFlags2 dstAccess);

void vkbGlobalBarrier(VkCommandBuffer cb, VkPipelineStageFlags2 srcStage, VkAccessFlags2 srcAccess,
                             VkPipelineStageFlags2 dstStage, VkAccessFlags2 dstAccess);

void vkbBeginRendering(VkCommandBuffer cb, uint32_t w, uint32_t h,
                              const VkImageView* colors, const int* loads, const float* clears, uint32_t nColor,
                              int hasDepth, VkImageView depth, int depthClear, float dclear,
                              VkImageLayout depthLayout);

void vkbCopyImageToBuffer(VkCommandBuffer cb, VkImage img, VkBuffer buf, uint32_t w, uint32_t h,
                                 uint32_t mip, uint32_t layer, VkImageAspectFlags aspect);

void vkbCopyBufferToImage(VkCommandBuffer cb, VkBuffer buf, uint64_t srcOffset, VkImage img,
                                 uint32_t w, uint32_t h, uint32_t mip, uint32_t layer, VkImageAspectFlags aspect);

void vkbPush(VkCommandBuffer cb, VkPipelineLayout layout, uint64_t root);

VkResult vkbAllocBuffer(VkDevice dev, VkPhysicalDevice phys, VkDeviceSize size, int hostVisible,
                               VkBuffer* outBuf, VkDeviceMemory* outMem, void** outPtr, VkDeviceAddress* outAddr);

void vkbFreeBuffer(VkDevice dev, VkBuffer buf, VkDeviceMemory mem);

VkResult vkbShaderModule(VkDevice dev, const uint32_t* code, size_t bytes, VkShaderModule* out);

VkResult vkbCreateComputePipeline(VkDevice dev, VkPipelineLayout layout,
                                         const void* code, size_t bytes, const char* entry, VkPipeline* out);

VkResult vkbCreateCreateGraphicsPipeline(VkDevice dev, VkPipelineLayout layout,
        const void* vs, size_t vsBytes, const void* fs, size_t fsBytes, const char* entry,
        VkPrimitiveTopology topo, const VkFormat* colorFmts, uint32_t nColor,
        VkFormat depthFmt, VkCullModeFlags cull, int frontFaceCW, int blendMode,
        int depthTest, int depthWrite, VkCompareOp depthCompare, uint32_t samples, VkPipeline* out);

VkQueryPool vkbCreateTimestampPool(VkDevice dev, uint32_t count);

VkResult vkbGetTimestamps(VkDevice dev, VkQueryPool pool, uint32_t count, uint64_t* out);

float vkbTimestampPeriod(VkPhysicalDevice pd);

uint32_t vkbTimestampValidBits(VkPhysicalDevice pd, uint32_t fam);

void vkbCmdResetQueryPool(VkCommandBuffer cb, VkQueryPool pool, uint32_t count);

void vkbCmdWriteTimestamp(VkCommandBuffer cb, VkPipelineStageFlags2 stage, VkQueryPool pool, uint32_t index);

VkSurfaceKHR vkbSurfaceFromHandle(uint64_t h);

VkResult vkbCreateSwapchain(VkPhysicalDevice phys, VkDevice dev, VkSurfaceKHR surface,
                                   uint32_t w, uint32_t h, VkSwapchainKHR old,
                                   VkSwapchainKHR* outSwap, VkFormat* outFmt, uint32_t* outW, uint32_t* outH);

uint32_t vkbSwapchainImageCount(VkDevice dev, VkSwapchainKHR swap);

void vkbSwapchainImages(VkDevice dev, VkSwapchainKHR swap, uint32_t n, VkImage* out);

VkResult vkbSwapImageView(VkDevice dev, VkImage img, VkFormat fmt, VkImageView* out);

VkResult vkbCreateSemaphore(VkDevice dev, VkSemaphore* out);

VkResult vkbAcquire(VkDevice dev, VkSwapchainKHR swap, VkSemaphore sem, uint32_t* outIndex);

void vkbPresentBarrier(VkCommandBuffer cb, VkImage img, VkImageLayout oldL);

VkResult vkbSubmitPresent(VkQueue q, VkCommandBuffer cb, VkSwapchainKHR swap, uint32_t imageIndex,
                                 VkSemaphore acquire, VkSemaphore renderDone, VkFence fence);

VkResult vkbCreateImage(VkDevice dev, VkPhysicalDevice phys,
                               VkFormat fmt, uint32_t w, uint32_t h, uint32_t layers, uint32_t mips,
                               VkImageUsageFlags usage, VkImageAspectFlags aspect, VkImageViewType viewType,
                               VkImage* outImg, VkDeviceMemory* outMem, VkImageView* outView);

void vkbWriteSampledImage(VkDevice dev, VkDescriptorSet set, uint32_t index, VkImageView view,
                                 VkImageLayout layout);

void vkbWriteStorageImage(VkDevice dev, VkDescriptorSet set, uint32_t index, VkImageView view);

VkResult vkbCreateSubView(VkDevice dev, VkImage img, VkFormat fmt, VkImageAspectFlags aspect,
                                 VkImageViewType viewType, uint32_t baseMip, uint32_t mipCount,
                                 uint32_t baseLayer, uint32_t layerCount, VkImageView* outView);

void vkbDestroyImage(VkDevice dev, VkImage img, VkDeviceMemory mem, VkImageView view);

VkResult vkbCreateMetalSurface(VkInstance inst, void* layer, VkSurfaceKHR* out);

VkResult vkbCreateXlibSurface(VkInstance inst, void* display, uint64_t window, VkSurfaceKHR* out);

VkResult vkbCreateWin32Surface(VkInstance inst, uintptr_t window, VkSurfaceKHR* out);

#ifdef __cplusplus
}
#endif

#endif
