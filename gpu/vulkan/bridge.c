#include "bridge.h"
#include <stdlib.h>
#include <string.h>

// Instance and device.
// createInstance targets Vulkan 1.3. Optional instance extensions (e.g. surface
// extensions from the windowing lib) are enabled when nExt > 0.
// TODO: make the application name configurable
// TODO: make allocator configurable
// TODO: Add VK_LAYER_KHRONOS_validation &&  VK_EXT_debug_utils, only enabled if is run in debug mode (configurable in the Renderer)
 VkResult vkbCreateInstance(uint32_t nExt, const char* const* exts, VkInstance* out) {
    VkApplicationInfo app = {0};
    app.sType = VK_STRUCTURE_TYPE_APPLICATION_INFO;
    app.pApplicationName = "gamekit";
    app.pEngineName = "gamekit";
    app.apiVersion = VK_API_VERSION_1_3;

    VkInstanceCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_INSTANCE_CREATE_INFO;
    ci.pApplicationInfo = &app;
    ci.enabledExtensionCount = nExt;
    ci.ppEnabledExtensionNames = exts;
    return vkCreateInstance(&ci, NULL, out);
}

// supportsUnifiedImageLayouts reports whether pd offers VK_KHR_unified_image_layouts
// with its unifiedImageLayouts feature: every image lives in VK_IMAGE_LAYOUT_GENERAL
// for its whole life, which is what lets the gpu API have no layouts at all.
static int vkbSupportsUnifiedImageLayouts(VkPhysicalDevice pd) {
    uint32_t n = 0;
    vkEnumerateDeviceExtensionProperties(pd, NULL, &n, NULL);
    VkExtensionProperties* exts = (VkExtensionProperties*)malloc(n * sizeof(VkExtensionProperties));
    vkEnumerateDeviceExtensionProperties(pd, NULL, &n, exts);
    int found = 0;
    for (uint32_t i = 0; i < n; i++) {
        if (strcmp(exts[i].extensionName, VK_KHR_UNIFIED_IMAGE_LAYOUTS_EXTENSION_NAME) == 0) {
            found = 1;
            break;
        }
    }
    free(exts);
    if (!found) {
        return 0;
    }

    VkPhysicalDeviceUnifiedImageLayoutsFeaturesKHR unified = {0};
    unified.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_UNIFIED_IMAGE_LAYOUTS_FEATURES_KHR;
    VkPhysicalDeviceFeatures2 f2 = {0};
    f2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2;
    f2.pNext = &unified;
    vkGetPhysicalDeviceFeatures2(pd, &f2);
    return unified.unifiedImageLayouts == VK_TRUE;
}

// pickPhysical chooses a device supporting unified image layouts, preferring a
// discrete GPU. VK_ERROR_FEATURE_NOT_PRESENT means devices exist but none supports them.
// TODO: allow the user to define what device they want
 VkResult vkbPickPhysical(VkInstance inst, VkPhysicalDevice* out) {
    uint32_t n = 0;
    vkEnumeratePhysicalDevices(inst, &n, NULL);

	if (n == 0) {
		return VK_ERROR_INITIALIZATION_FAILED;
	}

    VkPhysicalDevice* devs = (VkPhysicalDevice*)malloc(n * sizeof(VkPhysicalDevice));
    vkEnumeratePhysicalDevices(inst, &n, devs);
    VkPhysicalDevice chosen = VK_NULL_HANDLE;

	for (uint32_t i = 0; i < n; i++) {
        if (!vkbSupportsUnifiedImageLayouts(devs[i])) {
            continue;
        }
        VkPhysicalDeviceProperties p;
        vkGetPhysicalDeviceProperties(devs[i], &p);
        if (chosen == VK_NULL_HANDLE || p.deviceType == VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU) {
			chosen = devs[i];
		}
        if (p.deviceType == VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU) {
			break;
		}
    }
    free(devs);

    if (chosen == VK_NULL_HANDLE) {
        return VK_ERROR_FEATURE_NOT_PRESENT;
    }
	*out = chosen;
    return VK_SUCCESS;
}

// graphicsQueueFamily returns a family index with graphics+compute, or 0xFFFFFFFF.
// TODO: switch to vkGetPhysicalDeviceQueueFamilyProperties2
 uint32_t vkbGraphicsQueueFamily(VkPhysicalDevice pd) {
    uint32_t n = 0;
    vkGetPhysicalDeviceQueueFamilyProperties(pd, &n, NULL);
    VkQueueFamilyProperties* q = (VkQueueFamilyProperties*)malloc(n * sizeof(VkQueueFamilyProperties));
    vkGetPhysicalDeviceQueueFamilyProperties(pd, &n, q);
    uint32_t fam = 0xFFFFFFFF;
    for (uint32_t i = 0; i < n; i++) {
        if ((q[i].queueFlags & VK_QUEUE_GRAPHICS_BIT) && (q[i].queueFlags & VK_QUEUE_COMPUTE_BIT)) {
			fam = i;
			break;
		}
    }
    free(q);
    return fam;
}

// createDevice enables the 1.3 feature chain the gpu relies on: buffer device
// address, descriptor indexing (bindless heaps), dynamic rendering, sync2 — and
// unified image layouts, which vkbPickPhysical has already checked for.
// TODO: when we move to multi-threading use dedicated transfer queues
 VkResult vkbCreateDevice(VkPhysicalDevice pd, uint32_t fam, VkDevice* outDev, VkQueue* outQueue) {
    float prio = 1.0f;
    VkDeviceQueueCreateInfo qi = {0};
    qi.sType = VK_STRUCTURE_TYPE_DEVICE_QUEUE_CREATE_INFO;
    qi.queueFamilyIndex = fam;
    qi.queueCount = 1;
    qi.pQueuePriorities = &prio;

    VkPhysicalDeviceVulkan12Features f12 = {0};
    f12.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_1_2_FEATURES;
    f12.bufferDeviceAddress = VK_TRUE;
    f12.descriptorIndexing = VK_TRUE;
    f12.runtimeDescriptorArray = VK_TRUE;
    f12.descriptorBindingPartiallyBound = VK_TRUE;
    f12.descriptorBindingVariableDescriptorCount = VK_TRUE;
    f12.descriptorBindingSampledImageUpdateAfterBind = VK_TRUE;
    f12.descriptorBindingStorageImageUpdateAfterBind = VK_TRUE;
    f12.descriptorBindingUpdateUnusedWhilePending = VK_TRUE;
    f12.shaderSampledImageArrayNonUniformIndexing = VK_TRUE;
    f12.shaderStorageImageArrayNonUniformIndexing = VK_TRUE;
    f12.scalarBlockLayout = VK_TRUE; // GL_EXT_scalar_block_layout in the GLSL
    f12.timelineSemaphore = VK_TRUE;

    VkPhysicalDeviceVulkan13Features f13 = {0};
    f13.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_1_3_FEATURES;
    f13.dynamicRendering = VK_TRUE;
    f13.synchronization2 = VK_TRUE;
    f12.pNext = &f13;

    VkPhysicalDeviceUnifiedImageLayoutsFeaturesKHR unified = {0};
    unified.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_UNIFIED_IMAGE_LAYOUTS_FEATURES_KHR;
    unified.unifiedImageLayouts = VK_TRUE;
    f13.pNext = &unified;

    // Base features: drawIndirectFirstInstance lets each indirect command set its
    // own firstInstance, so one multi-draw-indirect call can cover many geometries
    // (gl_InstanceIndex directly indexes the compacted visible buffer).
    VkPhysicalDeviceFeatures2 f2 = {0};
    f2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_FEATURES_2;
    f2.features.drawIndirectFirstInstance = VK_TRUE;
    f2.features.multiDrawIndirect = VK_TRUE;
    // Anisotropic filtering is a base feature and must be requested explicitly,
    // or creating a sampler with anisotropyEnable is invalid.
    f2.features.samplerAnisotropy = VK_TRUE;
    // 64-bit integers in shaders. A device address held in a buffer is a
    // PhysicalStorageBuffer pointer and needs nothing extra, but the same address
    // held in PUSH CONSTANTS is loaded as a plain 64-bit scalar, which makes the
    // compiler emit the Int64 capability. Without this, every shader with an address
    // in its root fails to create — silently, as a blank frame rather than an error.
    f2.features.shaderInt64 = VK_TRUE;
    // Storage images are declared without a format qualifier, so one array serves every
    // format; writing through one needs this. Reads go through sampling instead.
    f2.features.shaderStorageImageWriteWithoutFormat = VK_TRUE;
    f2.pNext = &f12;

    const char* devExts[] = { VK_KHR_SWAPCHAIN_EXTENSION_NAME, VK_KHR_UNIFIED_IMAGE_LAYOUTS_EXTENSION_NAME };

    VkDeviceCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_DEVICE_CREATE_INFO;
    ci.pNext = &f2; // features2 chain (pEnabledFeatures must be NULL when using it)
    ci.queueCreateInfoCount = 1;
    ci.pQueueCreateInfos = &qi;
    ci.enabledExtensionCount = 2;
    ci.ppEnabledExtensionNames = devExts;

    VkResult r = vkCreateDevice(pd, &ci, NULL, outDev);
    if (r != VK_SUCCESS) {
		return r;
	}

    vkGetDeviceQueue(*outDev, fam, 0, outQueue);
    return VK_SUCCESS;
}

 void vkbDeviceName(VkPhysicalDevice pd, char* out) {
    VkPhysicalDeviceProperties p;
    vkGetPhysicalDeviceProperties(pd, &p);
    memcpy(out, p.deviceName, VK_MAX_PHYSICAL_DEVICE_NAME_SIZE);
}

 uint32_t vkbMaxPushConstantsSize(VkPhysicalDevice pd) {
    VkPhysicalDeviceProperties p;
    vkGetPhysicalDeviceProperties(pd, &p);
    return p.limits.maxPushConstantsSize;
}

 float vkbMaxAnisotropy(VkPhysicalDevice pd) {
    VkPhysicalDeviceProperties p;
    vkGetPhysicalDeviceProperties(pd, &p);
    return p.limits.maxSamplerAnisotropy;
}

 uint32_t vkbApiVersion(VkPhysicalDevice pd) {
    VkPhysicalDeviceProperties p;
    vkGetPhysicalDeviceProperties(pd, &p);
    return p.apiVersion;
}

// Bindless descriptors and samplers.
// Bindless heap binding slots (set 0). One global set, shared by every pipeline.
enum {
    BIND_SAMPLED = 0, // texture2D[]  (sampled images)
    BIND_STORAGE = 1, // image2D[]    (storage images)
    BIND_SAMPLER = 2, // sampler[]
};

static uint32_t vkbMin3(uint32_t a, uint32_t b, uint32_t c) {
    uint32_t m = a < b ? a : b;
    return m < c ? m : c;
}

// vkbCreateBindlessHeap builds the one descriptor set layout (3 update-after-bind
// arrays), pool, persistent set, and the pipeline layout every pipeline shares
// (that set + a push-constant holding the 64-bit root pointer). Capacities are
// clamped to the device's update-after-bind limits; the clamped values are
// returned so the Go side can bound its slot allocators.
 VkResult vkbCreateBindlessHeap(VkDevice dev, VkPhysicalDevice phys,
                                      uint32_t wantSampled, uint32_t wantStorage, uint32_t wantSampler,
                                      VkDescriptorSetLayout* outLayout, VkDescriptorPool* outPool,
                                      VkDescriptorSet* outSet, VkPipelineLayout* outPipeLayout,
                                      uint32_t* capSampled, uint32_t* capStorage, uint32_t* capSampler) {
    VkPhysicalDeviceVulkan12Properties p12 = {0};
    p12.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_VULKAN_1_2_PROPERTIES;
    VkPhysicalDeviceProperties2 p2 = {0};
    p2.sType = VK_STRUCTURE_TYPE_PHYSICAL_DEVICE_PROPERTIES_2;
    p2.pNext = &p12;
    vkGetPhysicalDeviceProperties2(phys, &p2);

    uint32_t nSampled = vkbMin3(wantSampled, p12.maxPerStageDescriptorUpdateAfterBindSampledImages, p12.maxDescriptorSetUpdateAfterBindSampledImages);
    uint32_t nStorage = vkbMin3(wantStorage, p12.maxPerStageDescriptorUpdateAfterBindStorageImages, p12.maxDescriptorSetUpdateAfterBindStorageImages);
    uint32_t nSampler = vkbMin3(wantSampler, p12.maxPerStageDescriptorUpdateAfterBindSamplers, p12.maxDescriptorSetUpdateAfterBindSamplers);
    *capSampled = nSampled;
    *capStorage = nStorage;
    *capSampler = nSampler;

    VkDescriptorSetLayoutBinding binds[3] = {0};
    binds[0].binding = BIND_SAMPLED; binds[0].descriptorType = VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE; binds[0].descriptorCount = nSampled; binds[0].stageFlags = VK_SHADER_STAGE_ALL;
    binds[1].binding = BIND_STORAGE; binds[1].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_IMAGE; binds[1].descriptorCount = nStorage; binds[1].stageFlags = VK_SHADER_STAGE_ALL;
    binds[2].binding = BIND_SAMPLER; binds[2].descriptorType = VK_DESCRIPTOR_TYPE_SAMPLER;       binds[2].descriptorCount = nSampler; binds[2].stageFlags = VK_SHADER_STAGE_ALL;

    VkDescriptorBindingFlags flags[3];
    for (int i = 0; i < 3; i++) flags[i] = VK_DESCRIPTOR_BINDING_UPDATE_AFTER_BIND_BIT | VK_DESCRIPTOR_BINDING_PARTIALLY_BOUND_BIT;
    VkDescriptorSetLayoutBindingFlagsCreateInfo bf = {0};
    bf.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_BINDING_FLAGS_CREATE_INFO;
    bf.bindingCount = 3;
    bf.pBindingFlags = flags;

    VkDescriptorSetLayoutCreateInfo lci = {0};
    lci.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
    lci.pNext = &bf;
    lci.flags = VK_DESCRIPTOR_SET_LAYOUT_CREATE_UPDATE_AFTER_BIND_POOL_BIT;
    lci.bindingCount = 3;
    lci.pBindings = binds;

    VkResult r = vkCreateDescriptorSetLayout(dev, &lci, NULL, outLayout);
    if (r != VK_SUCCESS) {
        return r;
    }

    VkDescriptorPoolSize sizes[3];
    sizes[0].type = VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE; sizes[0].descriptorCount = nSampled;
    sizes[1].type = VK_DESCRIPTOR_TYPE_STORAGE_IMAGE; sizes[1].descriptorCount = nStorage;
    sizes[2].type = VK_DESCRIPTOR_TYPE_SAMPLER;       sizes[2].descriptorCount = nSampler;
    VkDescriptorPoolCreateInfo pci = {0};
    pci.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
    pci.flags = VK_DESCRIPTOR_POOL_CREATE_UPDATE_AFTER_BIND_BIT;
    pci.maxSets = 1;
    pci.poolSizeCount = 3;
    pci.pPoolSizes = sizes;

    r = vkCreateDescriptorPool(dev, &pci, NULL, outPool);
    if (r != VK_SUCCESS) {
        vkDestroyDescriptorSetLayout(dev, *outLayout, NULL);
        return r;
    }

    VkDescriptorSetAllocateInfo sai = {0};
    sai.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
    sai.descriptorPool = *outPool;
    sai.descriptorSetCount = 1;
    sai.pSetLayouts = outLayout;

    r = vkAllocateDescriptorSets(dev, &sai, outSet);
    if (r != VK_SUCCESS) {
        vkDestroyDescriptorPool(dev, *outPool, NULL);
        vkDestroyDescriptorSetLayout(dev, *outLayout, NULL);
        return r;
    }

    // Every pipeline shares this layout, with a push constant range sized to what the
    // device actually permits rather than a fixed guess. It was 128 bytes, which is
    // smaller than some roots callers legitimately want to pass — and the limit is a
    // driver policy decision, not a hardware one (the same Apple GPU reports 4096
    // through MoltenVK and 256 through KosmicKrisp), so it has to be queried.
    VkPhysicalDeviceProperties props;
    vkGetPhysicalDeviceProperties(phys, &props);
    VkPushConstantRange pcr = {0};
    pcr.stageFlags = VK_SHADER_STAGE_ALL;
    pcr.offset = 0;
    pcr.size = props.limits.maxPushConstantsSize;
    VkPipelineLayoutCreateInfo plci = {0};
    plci.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
    plci.setLayoutCount = 1;
    plci.pSetLayouts = outLayout;
    plci.pushConstantRangeCount = 1;
    plci.pPushConstantRanges = &pcr;


    r = vkCreatePipelineLayout(dev, &plci, NULL, outPipeLayout);
    if (r != VK_SUCCESS) {
        vkDestroyDescriptorPool(dev, *outPool, NULL); // frees the allocated set too
        vkDestroyDescriptorSetLayout(dev, *outLayout, NULL);
        return r;
    }
    return VK_SUCCESS;
}

 VkResult vkbCreateSampler(VkDevice dev, VkFilter mag, VkFilter min, VkSamplerMipmapMode mip,
                                 VkSamplerAddressMode u, VkSamplerAddressMode v, VkSamplerAddressMode w,
                                 int compareEnable, VkCompareOp compareOp, float maxAniso, VkSampler* out) {
    VkSamplerCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_SAMPLER_CREATE_INFO;
    ci.magFilter = mag; ci.minFilter = min; ci.mipmapMode = mip;
    ci.addressModeU = u; ci.addressModeV = v; ci.addressModeW = w;
    ci.minLod = 0.0f; ci.maxLod = VK_LOD_CLAMP_NONE;
    ci.compareEnable = compareEnable ? VK_TRUE : VK_FALSE;
    ci.compareOp = compareOp;
    ci.anisotropyEnable = maxAniso > 1.0f ? VK_TRUE : VK_FALSE;
    ci.maxAnisotropy = maxAniso < 1.0f ? 1.0f : maxAniso;
    return vkCreateSampler(dev, &ci, NULL, out);
}

 void vkbWriteSampler(VkDevice dev, VkDescriptorSet set, uint32_t index, VkSampler s) {
    VkDescriptorImageInfo ii = {0};
    ii.sampler = s;
    VkWriteDescriptorSet w = {0};
    w.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    w.dstSet = set;
    w.dstBinding = BIND_SAMPLER;
    w.dstArrayElement = index;
    w.descriptorCount = 1;
    w.descriptorType = VK_DESCRIPTOR_TYPE_SAMPLER;
    w.pImageInfo = &ii;
    vkUpdateDescriptorSets(dev, 1, &w, 0, NULL);
}

// Command recording and synchronization.
// Vulkan synchronization2 flags are 64-bit macros. Referencing them directly
// from Go makes some cgo/header combinations emit unresolved data symbols, so
// expose their values through C functions instead.
 VkPipelineStageFlags2 vkbStageNone(void) { return VK_PIPELINE_STAGE_2_NONE; }
 VkPipelineStageFlags2 vkbStageTop(void) { return VK_PIPELINE_STAGE_2_TOP_OF_PIPE_BIT; }
 VkPipelineStageFlags2 vkbStageIndirect(void) { return VK_PIPELINE_STAGE_2_DRAW_INDIRECT_BIT; }
 VkPipelineStageFlags2 vkbStageVertex(void) { return VK_PIPELINE_STAGE_2_VERTEX_SHADER_BIT; }
 VkPipelineStageFlags2 vkbStageFragment(void) { return VK_PIPELINE_STAGE_2_FRAGMENT_SHADER_BIT; }
 VkPipelineStageFlags2 vkbStageColor(void) { return VK_PIPELINE_STAGE_2_COLOR_ATTACHMENT_OUTPUT_BIT; }
 VkPipelineStageFlags2 vkbStageEarlyDepth(void) { return VK_PIPELINE_STAGE_2_EARLY_FRAGMENT_TESTS_BIT; }
 VkPipelineStageFlags2 vkbStageLateDepth(void) { return VK_PIPELINE_STAGE_2_LATE_FRAGMENT_TESTS_BIT; }
 VkPipelineStageFlags2 vkbStageCompute(void) { return VK_PIPELINE_STAGE_2_COMPUTE_SHADER_BIT; }
 VkPipelineStageFlags2 vkbStageTransfer(void) { return VK_PIPELINE_STAGE_2_ALL_TRANSFER_BIT; }
 VkPipelineStageFlags2 vkbStageAll(void) { return VK_PIPELINE_STAGE_2_ALL_COMMANDS_BIT; }

 VkAccessFlags2 vkbAccessNone(void) { return VK_ACCESS_2_NONE; }
 VkAccessFlags2 vkbAccessIndirectRead(void) { return VK_ACCESS_2_INDIRECT_COMMAND_READ_BIT; }
 VkAccessFlags2 vkbAccessShaderRead(void) { return VK_ACCESS_2_SHADER_READ_BIT; }
 VkAccessFlags2 vkbAccessShaderWrite(void) { return VK_ACCESS_2_SHADER_WRITE_BIT; }
 VkAccessFlags2 vkbAccessColorRead(void) { return VK_ACCESS_2_COLOR_ATTACHMENT_READ_BIT; }
 VkAccessFlags2 vkbAccessColorWrite(void) { return VK_ACCESS_2_COLOR_ATTACHMENT_WRITE_BIT; }
 VkAccessFlags2 vkbAccessDepthRead(void) { return VK_ACCESS_2_DEPTH_STENCIL_ATTACHMENT_READ_BIT; }
 VkAccessFlags2 vkbAccessDepthWrite(void) { return VK_ACCESS_2_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT; }
 VkAccessFlags2 vkbAccessTransferRead(void) { return VK_ACCESS_2_TRANSFER_READ_BIT; }
 VkAccessFlags2 vkbAccessTransferWrite(void) { return VK_ACCESS_2_TRANSFER_WRITE_BIT; }
 VkAccessFlags2 vkbAccessMemoryRead(void) { return VK_ACCESS_2_MEMORY_READ_BIT; }
 VkAccessFlags2 vkbAccessMemoryWrite(void) { return VK_ACCESS_2_MEMORY_WRITE_BIT; }

 VkResult vkbCreateCommandPool(VkDevice dev, uint32_t fam, VkCommandPool* out) {
    VkCommandPoolCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_COMMAND_POOL_CREATE_INFO;
    ci.flags = VK_COMMAND_POOL_CREATE_RESET_COMMAND_BUFFER_BIT;
    ci.queueFamilyIndex = fam;
    return vkCreateCommandPool(dev, &ci, NULL, out);
}

 VkResult vkbAllocCmd(VkDevice dev, VkCommandPool pool, VkCommandBuffer* out) {
    VkCommandBufferAllocateInfo ai = {0};
    ai.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
    ai.commandPool = pool;
    ai.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
    ai.commandBufferCount = 1;
    return vkAllocateCommandBuffers(dev, &ai, out);
}

 VkResult vkbBeginCmd(VkCommandBuffer cb) {
    VkCommandBufferBeginInfo bi = {0};
    bi.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    bi.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    return vkBeginCommandBuffer(cb, &bi);
}

// Bind the global bindless set at both bind points for the whole recording.
 void vkbBindHeap(VkCommandBuffer cb, VkPipelineLayout layout, VkDescriptorSet set) {
    vkCmdBindDescriptorSets(cb, VK_PIPELINE_BIND_POINT_GRAPHICS, layout, 0, 1, &set, 0, NULL);
    vkCmdBindDescriptorSets(cb, VK_PIPELINE_BIND_POINT_COMPUTE, layout, 0, 1, &set, 0, NULL);
}

 VkResult vkbSubmit(VkDevice dev, VkQueue q, VkCommandBuffer cb, VkFence* outFence) {
    VkFenceCreateInfo fi = {0};
    fi.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
    VkResult r = vkCreateFence(dev, &fi, NULL, outFence);
    if (r != VK_SUCCESS) return r;
    VkCommandBufferSubmitInfo cbi = {0};
    cbi.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_SUBMIT_INFO;
    cbi.commandBuffer = cb;
    VkSubmitInfo2 si = {0};
    si.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO_2;
    si.commandBufferInfoCount = 1;
    si.pCommandBufferInfos = &cbi;
    return vkQueueSubmit2(q, 1, &si, *outFence);
}

// vkbInitializeImage moves a new image, or a freshly acquired backbuffer, out of
// VK_IMAGE_LAYOUT_UNDEFINED into VK_IMAGE_LAYOUT_GENERAL, where it stays. Its contents
// are discarded. The source scope is every stage, so the barrier also chains with an
// acquire semaphore's wait.
 void vkbInitializeImage(VkCommandBuffer cb, VkImage img, VkImageAspectFlags aspect) {
    VkImageMemoryBarrier2 ib = {0};
    ib.sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER_2;
    ib.srcStageMask = VK_PIPELINE_STAGE_2_ALL_COMMANDS_BIT; ib.srcAccessMask = VK_ACCESS_2_NONE;
    ib.dstStageMask = VK_PIPELINE_STAGE_2_ALL_COMMANDS_BIT;
    ib.dstAccessMask = VK_ACCESS_2_MEMORY_READ_BIT | VK_ACCESS_2_MEMORY_WRITE_BIT;
    ib.oldLayout = VK_IMAGE_LAYOUT_UNDEFINED; ib.newLayout = VK_IMAGE_LAYOUT_GENERAL;
    ib.image = img;
    ib.subresourceRange.aspectMask = aspect;
    ib.subresourceRange.levelCount = VK_REMAINING_MIP_LEVELS;
    ib.subresourceRange.layerCount = VK_REMAINING_ARRAY_LAYERS;
    VkDependencyInfo di = {0};
    di.sType = VK_STRUCTURE_TYPE_DEPENDENCY_INFO;
    di.imageMemoryBarrierCount = 1;
    di.pImageMemoryBarriers = &ib;
    vkCmdPipelineBarrier2(cb, &di);
}

// vkbGlobalBarrier is the gpu.Barrier: a queue+stage memory barrier, no resources.
 void vkbGlobalBarrier(VkCommandBuffer cb, VkPipelineStageFlags2 srcStage, VkAccessFlags2 srcAccess,
                             VkPipelineStageFlags2 dstStage, VkAccessFlags2 dstAccess) {
    VkMemoryBarrier2 mb = {0};
    mb.sType = VK_STRUCTURE_TYPE_MEMORY_BARRIER_2;
    mb.srcStageMask = srcStage; mb.srcAccessMask = srcAccess;
    mb.dstStageMask = dstStage; mb.dstAccessMask = dstAccess;
    VkDependencyInfo di = {0};
    di.sType = VK_STRUCTURE_TYPE_DEPENDENCY_INFO;
    di.memoryBarrierCount = 1;
    di.pMemoryBarriers = &mb;
    vkCmdPipelineBarrier2(cb, &di);
}

// vkbBeginRendering starts dynamic rendering with up to 4 color attachments (0 for a
// depth-only pass, e.g. a shadow map; up to 4 for MRT, e.g. the G-buffer fill) and an
// optional depth attachment. colors/colorResolves/loads/clears are parallel arrays of
// length nColor (clears is nColor*4 floats, rgba per attachment). A non-null resolve
// view receives its multisampled attachment's samples when the pass ends: averaged for
// colour, sample zero's for depth, the one depth resolve every device supports. Every
// attachment is in VK_IMAGE_LAYOUT_GENERAL, as every image is.
 void vkbBeginRendering(VkCommandBuffer cb, uint32_t w, uint32_t h,
                              const VkImageView* colors, const VkImageView* colorResolves, const int* loads, const int* transients, const float* clears, uint32_t nColor,
                              int hasDepth, VkImageView depth, VkImageView depthResolve, int depthClear, int depthTransient, float dclear) {
    VkRenderingAttachmentInfo cis[4] = {0};
    for (uint32_t i = 0; i < nColor; i++) {
        cis[i].sType = VK_STRUCTURE_TYPE_RENDERING_ATTACHMENT_INFO;
        cis[i].imageView = colors[i];
        cis[i].imageLayout = VK_IMAGE_LAYOUT_GENERAL;
        if (colorResolves[i] != VK_NULL_HANDLE) {
            cis[i].resolveMode = VK_RESOLVE_MODE_AVERAGE_BIT;
            cis[i].resolveImageView = colorResolves[i];
            cis[i].resolveImageLayout = VK_IMAGE_LAYOUT_GENERAL;
        }
        // A transient attachment is never loaded or stored (gpu.TextureTransient): with
        // lazily allocated memory, either would make the device give it some.
        cis[i].loadOp = loads[i] ? VK_ATTACHMENT_LOAD_OP_CLEAR : transients[i] ? VK_ATTACHMENT_LOAD_OP_DONT_CARE : VK_ATTACHMENT_LOAD_OP_LOAD;
        cis[i].storeOp = transients[i] ? VK_ATTACHMENT_STORE_OP_DONT_CARE : VK_ATTACHMENT_STORE_OP_STORE;
        cis[i].clearValue.color.float32[0] = clears[i * 4 + 0];
        cis[i].clearValue.color.float32[1] = clears[i * 4 + 1];
        cis[i].clearValue.color.float32[2] = clears[i * 4 + 2];
        cis[i].clearValue.color.float32[3] = clears[i * 4 + 3];
    }

    VkRenderingAttachmentInfo di = {0};
    di.sType = VK_STRUCTURE_TYPE_RENDERING_ATTACHMENT_INFO;
    di.imageView = depth;
    di.imageLayout = VK_IMAGE_LAYOUT_GENERAL;
    if (depthResolve != VK_NULL_HANDLE) {
        di.resolveMode = VK_RESOLVE_MODE_SAMPLE_ZERO_BIT;
        di.resolveImageView = depthResolve;
        di.resolveImageLayout = VK_IMAGE_LAYOUT_GENERAL;
    }
    di.loadOp = depthClear ? VK_ATTACHMENT_LOAD_OP_CLEAR : depthTransient ? VK_ATTACHMENT_LOAD_OP_DONT_CARE : VK_ATTACHMENT_LOAD_OP_LOAD;
    di.storeOp = depthTransient ? VK_ATTACHMENT_STORE_OP_DONT_CARE : VK_ATTACHMENT_STORE_OP_STORE;
    di.clearValue.depthStencil.depth = dclear;

    VkRenderingInfo ri = {0};
    ri.sType = VK_STRUCTURE_TYPE_RENDERING_INFO;
    ri.renderArea.extent.width = w; ri.renderArea.extent.height = h;
    ri.layerCount = 1;
    ri.colorAttachmentCount = nColor;
    ri.pColorAttachments = nColor > 0 ? cis : NULL;
    if (hasDepth) ri.pDepthAttachment = &di;
    vkCmdBeginRendering(cb, &ri);
}

 void vkbCopyImageToBuffer(VkCommandBuffer cb, VkImage img, VkBuffer buf, uint32_t w, uint32_t h, uint32_t d,
                                 uint32_t mip, uint32_t layer, VkImageAspectFlags aspect) {
    VkBufferImageCopy c = {0};
    c.imageSubresource.aspectMask = aspect;
    c.imageSubresource.mipLevel = mip;
    c.imageSubresource.baseArrayLayer = layer;
    c.imageSubresource.layerCount = 1;
    c.imageExtent.width = w; c.imageExtent.height = h; c.imageExtent.depth = d;
    vkCmdCopyImageToBuffer(cb, img, VK_IMAGE_LAYOUT_GENERAL, buf, 1, &c);
}

 void vkbCopyBufferToImage(VkCommandBuffer cb, VkBuffer buf, uint64_t srcOffset, VkImage img,
                                 uint32_t w, uint32_t h, uint32_t d, uint32_t mip, uint32_t layer, VkImageAspectFlags aspect) {
    VkBufferImageCopy c = {0};
    c.bufferOffset = srcOffset;
    c.imageSubresource.aspectMask = aspect;
    c.imageSubresource.mipLevel = mip;
    c.imageSubresource.baseArrayLayer = layer;
    c.imageSubresource.layerCount = 1;
    c.imageExtent.width = w; c.imageExtent.height = h; c.imageExtent.depth = d;
    vkCmdCopyBufferToImage(cb, buf, img, VK_IMAGE_LAYOUT_GENERAL, 1, &c);
}

 void vkbPush(VkCommandBuffer cb, VkPipelineLayout layout, const void* data, uint32_t size) {
    if (!data || size == 0) return;
    vkCmdPushConstants(cb, layout, VK_SHADER_STAGE_ALL, 0, size, data);
}

// Buffer memory.
// vkbAllocBuffer creates a buffer + backing memory (one dedicated allocation),
// binds them, maps host-visible memory, and returns the device address. Buffers
// carry every usage the bindless model needs, so a single Alloc serves as
// storage / index / indirect / transfer / uniform and exposes a BDA.
 VkResult vkbAllocBuffer(VkDevice dev, VkPhysicalDevice phys, VkDeviceSize size, int hostVisible,
                               VkBuffer* outBuf, VkDeviceMemory* outMem, void** outPtr, VkDeviceAddress* outAddr) {
    VkBufferCreateInfo bi = {0};
    bi.sType = VK_STRUCTURE_TYPE_BUFFER_CREATE_INFO;
    bi.size = size;
    bi.usage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_UNIFORM_BUFFER_BIT |
               VK_BUFFER_USAGE_INDEX_BUFFER_BIT | VK_BUFFER_USAGE_INDIRECT_BUFFER_BIT |
               VK_BUFFER_USAGE_TRANSFER_SRC_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT |
               VK_BUFFER_USAGE_SHADER_DEVICE_ADDRESS_BIT;
    bi.sharingMode = VK_SHARING_MODE_EXCLUSIVE;

    VkResult r = vkCreateBuffer(dev, &bi, NULL, outBuf);
    if (r != VK_SUCCESS) {
        return r;
    }

    VkMemoryRequirements req;
    vkGetBufferMemoryRequirements(dev, *outBuf, &req);

    VkPhysicalDeviceMemoryProperties mp;
    vkGetPhysicalDeviceMemoryProperties(phys, &mp);
    VkMemoryPropertyFlags want = hostVisible
        ? (VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT)
        : VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT;
    uint32_t typeIndex = 0xFFFFFFFF;
    for (uint32_t i = 0; i < mp.memoryTypeCount; i++) {
        if ((req.memoryTypeBits & (1u << i)) && (mp.memoryTypes[i].propertyFlags & want) == want) {
            typeIndex = i;
            break;
        }
    }
    if (typeIndex == 0xFFFFFFFF) {
        vkDestroyBuffer(dev, *outBuf, NULL);
        return VK_ERROR_OUT_OF_DEVICE_MEMORY;
    }

    VkMemoryAllocateFlagsInfo fi = {0};
    fi.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_FLAGS_INFO;
    fi.flags = VK_MEMORY_ALLOCATE_DEVICE_ADDRESS_BIT;

    VkMemoryAllocateInfo ai = {0};
    ai.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
    ai.pNext = &fi;
    ai.allocationSize = req.size;
    ai.memoryTypeIndex = typeIndex;

    r = vkAllocateMemory(dev, &ai, NULL, outMem);
    if (r != VK_SUCCESS) {
        vkDestroyBuffer(dev, *outBuf, NULL);
        return r;
    }

    r = vkBindBufferMemory(dev, *outBuf, *outMem, 0);
    if (r != VK_SUCCESS) {
        vkFreeMemory(dev, *outMem, NULL); vkDestroyBuffer(dev, *outBuf, NULL);
        return r;
    }

    if (hostVisible) {
        r = vkMapMemory(dev, *outMem, 0, VK_WHOLE_SIZE, 0, outPtr);
        if (r != VK_SUCCESS) {
            vkFreeMemory(dev, *outMem, NULL); vkDestroyBuffer(dev, *outBuf, NULL);
            return r;
        }
    } else {
        *outPtr = NULL;
    }

    VkBufferDeviceAddressInfo dai = {0};
    dai.sType = VK_STRUCTURE_TYPE_BUFFER_DEVICE_ADDRESS_INFO;
    dai.buffer = *outBuf;
    *outAddr = vkGetBufferDeviceAddress(dev, &dai);
    return VK_SUCCESS;
}

 void vkbFreeBuffer(VkDevice dev, VkBuffer buf, VkDeviceMemory mem) {
    vkDestroyBuffer(dev, buf, NULL);
    vkFreeMemory(dev, mem, NULL);
}

// Graphics and compute pipelines.
 VkResult vkbShaderModule(VkDevice dev, const uint32_t* code, size_t bytes, VkShaderModule* out) {
    VkShaderModuleCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
    ci.codeSize = bytes;
    ci.pCode = code;
    return vkCreateShaderModule(dev, &ci, NULL, out);
}

// specialization describes a pipeline's specialization constants to Vulkan: constant
// ids[i] takes the 32 bits bits[i]. Every constant is 4 bytes, VkBool32 included. The
// entries are allocated, and released with freeSpecialization; count 0 allocates none.
static VkSpecializationInfo specialization(const uint32_t* ids, const uint32_t* bits, uint32_t count) {
    VkSpecializationInfo info = {0};
    if (count == 0) return info;
    VkSpecializationMapEntry* entries = calloc(count, sizeof(VkSpecializationMapEntry));
    for (uint32_t i = 0; i < count; i++) {
        entries[i].constantID = ids[i];
        entries[i].offset = i * sizeof(uint32_t);
        entries[i].size = sizeof(uint32_t);
    }
    info.mapEntryCount = count;
    info.pMapEntries = entries;
    info.dataSize = count * sizeof(uint32_t);
    info.pData = bits;
    return info;
}

static void freeSpecialization(VkSpecializationInfo* info) {
    free((void*)info->pMapEntries);
}

 VkResult vkbCreateComputePipeline(VkDevice dev, VkPipelineLayout layout,
                                         const void* code, size_t bytes, const char* entry,
                                         const uint32_t* constantIDs, const uint32_t* constantBits, uint32_t constantCount,
                                         VkPipeline* out) {
    VkShaderModule mod;
    VkResult r = vkbShaderModule(dev, (const uint32_t*)code, bytes, &mod);
    if (r != VK_SUCCESS) return r;
    VkSpecializationInfo spec = specialization(constantIDs, constantBits, constantCount);

    VkComputePipelineCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
    ci.layout = layout;
    ci.stage.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    ci.stage.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    ci.stage.module = mod;
    ci.stage.pName = entry;
    ci.stage.pSpecializationInfo = &spec;

    r = vkCreateComputePipelines(dev, VK_NULL_HANDLE, 1, &ci, NULL, out);
    vkDestroyShaderModule(dev, mod, NULL);
    freeSpecialization(&spec);
    return r;
}

// vkbCreateGraphicsPipeline builds a minimal graphics pipeline: vs (+fs), empty
// vertex input (geometry is pulled via BDA), dynamic viewport/scissor, dynamic
// rendering (formats via VkPipelineRenderingCreateInfo, renderPass = NULL),
// opaque color targets. Depth is enabled iff depthFormat != UNDEFINED.
//
// fs may be NULL, for a pipeline that writes depth and nothing else. That is a real
// configuration rather than a shortcut: a fragment stage bound to a pass with no colour
// attachment still runs, and one bound to a pass that HAS an attachment while writing
// nothing to it produces undefined colour rather than no colour.
 VkResult vkbCreateCreateGraphicsPipeline(VkDevice dev, VkPipelineLayout layout,
        const void* vs, size_t vsBytes, const void* fs, size_t fsBytes, const char* entry,
        VkPrimitiveTopology topo, const VkFormat* colorFmts, uint32_t nColor,
        VkFormat depthFmt, VkCullModeFlags cull, int frontFaceCW, const VkPipelineColorBlendAttachmentState* blends,
        int depthTest, int depthWrite, VkCompareOp depthCompare, uint32_t samples,
        const uint32_t* constantIDs, const uint32_t* constantBits, uint32_t constantCount, VkPipeline* out) {
    VkShaderModule vmod, fmod = VK_NULL_HANDLE;
    VkResult r = vkbShaderModule(dev, (const uint32_t*)vs, vsBytes, &vmod);
    if (r != VK_SUCCESS) return r;
    uint32_t nStages = 1;
    if (fs && fsBytes) {
        r = vkbShaderModule(dev, (const uint32_t*)fs, fsBytes, &fmod);
        if (r != VK_SUCCESS) { vkDestroyShaderModule(dev, vmod, NULL); return r; }
        nStages = 2;
    }

    VkPipelineShaderStageCreateInfo stages[2] = {0};
    stages[0].sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stages[0].stage = VK_SHADER_STAGE_VERTEX_BIT; stages[0].module = vmod; stages[0].pName = entry;
    stages[1].sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stages[1].stage = VK_SHADER_STAGE_FRAGMENT_BIT; stages[1].module = fmod; stages[1].pName = entry;
    // Both stages share one set of constants; a stage ignores any it does not declare.
    VkSpecializationInfo spec = specialization(constantIDs, constantBits, constantCount);
    stages[0].pSpecializationInfo = &spec;
    stages[1].pSpecializationInfo = &spec;

    VkPipelineVertexInputStateCreateInfo vin = {0};
    vin.sType = VK_STRUCTURE_TYPE_PIPELINE_VERTEX_INPUT_STATE_CREATE_INFO;

    VkPipelineInputAssemblyStateCreateInfo ia = {0};
    ia.sType = VK_STRUCTURE_TYPE_PIPELINE_INPUT_ASSEMBLY_STATE_CREATE_INFO;
    ia.topology = topo;

    VkPipelineViewportStateCreateInfo vp = {0};
    vp.sType = VK_STRUCTURE_TYPE_PIPELINE_VIEWPORT_STATE_CREATE_INFO;
    vp.viewportCount = 1; vp.scissorCount = 1; // dynamic

    VkPipelineRasterizationStateCreateInfo rs = {0};
    rs.sType = VK_STRUCTURE_TYPE_PIPELINE_RASTERIZATION_STATE_CREATE_INFO;
    rs.polygonMode = VK_POLYGON_MODE_FILL;
    rs.cullMode = cull;
    rs.frontFace = frontFaceCW ? VK_FRONT_FACE_CLOCKWISE : VK_FRONT_FACE_COUNTER_CLOCKWISE;
    rs.lineWidth = 1.0f;
    // Depth bias is always enabled and its factors are dynamic (SetDepthBias);
    // a pipeline that never calls it just runs with the all-zero default,
    // equivalent to bias disabled.
    rs.depthBiasEnable = VK_TRUE;

    VkPipelineMultisampleStateCreateInfo ms = {0};
    ms.sType = VK_STRUCTURE_TYPE_PIPELINE_MULTISAMPLE_STATE_CREATE_INFO;
    ms.rasterizationSamples = samples < 2 ? VK_SAMPLE_COUNT_1_BIT : (VkSampleCountFlagBits)samples;

    VkPipelineDepthStencilStateCreateInfo ds = {0};
    ds.sType = VK_STRUCTURE_TYPE_PIPELINE_DEPTH_STENCIL_STATE_CREATE_INFO;
    ds.depthTestEnable = depthTest ? VK_TRUE : VK_FALSE;
    ds.depthWriteEnable = depthWrite ? VK_TRUE : VK_FALSE;
    ds.depthCompareOp = depthCompare;

    // blends holds one state per colour attachment, built from gpu.BlendState.
    VkPipelineColorBlendStateCreateInfo cb = {0};
    cb.sType = VK_STRUCTURE_TYPE_PIPELINE_COLOR_BLEND_STATE_CREATE_INFO;
    cb.attachmentCount = nColor;
    cb.pAttachments = blends;

    VkDynamicState dyn[3] = { VK_DYNAMIC_STATE_VIEWPORT, VK_DYNAMIC_STATE_SCISSOR, VK_DYNAMIC_STATE_DEPTH_BIAS };
    VkPipelineDynamicStateCreateInfo dsi = {0};
    dsi.sType = VK_STRUCTURE_TYPE_PIPELINE_DYNAMIC_STATE_CREATE_INFO;
    dsi.dynamicStateCount = 3; dsi.pDynamicStates = dyn;

    VkPipelineRenderingCreateInfo rci = {0};
    rci.sType = VK_STRUCTURE_TYPE_PIPELINE_RENDERING_CREATE_INFO;
    rci.colorAttachmentCount = nColor;
    rci.pColorAttachmentFormats = colorFmts;
    rci.depthAttachmentFormat = depthFmt;

    VkGraphicsPipelineCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_GRAPHICS_PIPELINE_CREATE_INFO;
    ci.pNext = &rci; // dynamic rendering
    ci.stageCount = nStages; ci.pStages = stages;
    ci.pVertexInputState = &vin;
    ci.pInputAssemblyState = &ia;
    ci.pViewportState = &vp;
    ci.pRasterizationState = &rs;
    ci.pMultisampleState = &ms;
    ci.pDepthStencilState = &ds;
    ci.pColorBlendState = &cb;
    ci.pDynamicState = &dsi;
    ci.layout = layout;
    ci.renderPass = VK_NULL_HANDLE; // dynamic rendering

    r = vkCreateGraphicsPipelines(dev, VK_NULL_HANDLE, 1, &ci, NULL, out);
    vkDestroyShaderModule(dev, vmod, NULL);
    if (fmod != VK_NULL_HANDLE) vkDestroyShaderModule(dev, fmod, NULL);
    freeSpecialization(&spec);
    return r;
}

// Timestamp queries.
 VkQueryPool vkbCreateTimestampPool(VkDevice dev, uint32_t count) {
    VkQueryPoolCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
    ci.queryType = VK_QUERY_TYPE_TIMESTAMP;
    ci.queryCount = count;
    VkQueryPool pool = VK_NULL_HANDLE;
    vkCreateQueryPool(dev, &ci, NULL, &pool);
    return pool;
}

// vkbGetTimestamps reads count 64-bit results into out. Returns VK_SUCCESS when all
// are available. WITH the WAIT bit it blocks until ready.
 VkResult vkbGetTimestamps(VkDevice dev, VkQueryPool pool, uint32_t count, uint64_t* out) {
    return vkGetQueryPoolResults(dev, pool, 0, count, count * sizeof(uint64_t), out,
        sizeof(uint64_t), VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT);
}

 float vkbTimestampPeriod(VkPhysicalDevice pd) {
    VkPhysicalDeviceProperties p;
    vkGetPhysicalDeviceProperties(pd, &p);
    return p.limits.timestampPeriod; // nanoseconds per tick
}

 uint32_t vkbTimestampValidBits(VkPhysicalDevice pd, uint32_t fam) {
    uint32_t n = 0;
    vkGetPhysicalDeviceQueueFamilyProperties(pd, &n, NULL);
    VkQueueFamilyProperties* q = (VkQueueFamilyProperties*)malloc(n * sizeof(VkQueueFamilyProperties));
    vkGetPhysicalDeviceQueueFamilyProperties(pd, &n, q);
    uint32_t bits = (fam < n) ? q[fam].timestampValidBits : 0;
    free(q);
    return bits;
}

 void vkbCmdResetQueryPool(VkCommandBuffer cb, VkQueryPool pool, uint32_t count) {
    vkCmdResetQueryPool(cb, pool, 0, count);
}

 void vkbCmdWriteTimestamp(VkCommandBuffer cb, VkPipelineStageFlags2 stage, VkQueryPool pool, uint32_t index) {
    vkCmdWriteTimestamp2(cb, stage, pool, index);
}

// Swapchains and presentation.
// vkbSurfaceFromHandle reinterprets an opaque handle value (the uintptr the HAL
// passes across the package boundary) as a VkSurfaceKHR. Doing the cast in C keeps
// the Go side free of a vet-flagged uintptr->unsafe.Pointer conversion; the value
// is a real Vulkan handle, never Go memory.
 VkSurfaceKHR vkbSurfaceFromHandle(uint64_t h) { return (VkSurfaceKHR)h; }
 uint64_t vkbSurfaceHandle(VkSurfaceKHR surface) { return (uint64_t)surface; }

// vkbSurfaceFormats writes up to max of the formats the surface presents in the sRGB
// colour space into out, and returns how many it wrote. Other colour spaces (HDR10,
// extended linear) are left out until a swapchain can ask for one.
 uint32_t vkbSurfaceFormats(VkPhysicalDevice phys, VkSurfaceKHR surface, VkFormat* out, uint32_t max) {
    uint32_t nf = 0;
    vkGetPhysicalDeviceSurfaceFormatsKHR(phys, surface, &nf, NULL);
    VkSurfaceFormatKHR* fmts = (VkSurfaceFormatKHR*)malloc(nf * sizeof(VkSurfaceFormatKHR));
    vkGetPhysicalDeviceSurfaceFormatsKHR(phys, surface, &nf, fmts);
    uint32_t n = 0;
    for (uint32_t i = 0; i < nf && n < max; i++) {
        if (fmts[i].colorSpace == VK_COLOR_SPACE_SRGB_NONLINEAR_KHR) {
            out[n++] = fmts[i].format;
        }
    }
    free(fmts);
    return n;
}

// vkbPresentMode is the present mode a swapchain asking for wanted gets: wanted, if the
// surface offers it; for IMMEDIATE, MAILBOX — which replaces the waiting frame rather
// than tearing — if it offers that instead; and otherwise FIFO, which every surface
// offers.
static VkPresentModeKHR vkbPresentMode(VkPhysicalDevice phys, VkSurfaceKHR surface, VkPresentModeKHR wanted) {
    if (wanted == VK_PRESENT_MODE_FIFO_KHR) {
        return VK_PRESENT_MODE_FIFO_KHR;
    }
    uint32_t n = 0;
    vkGetPhysicalDeviceSurfacePresentModesKHR(phys, surface, &n, NULL);
    VkPresentModeKHR* modes = malloc(n * sizeof(VkPresentModeKHR));
    vkGetPhysicalDeviceSurfacePresentModesKHR(phys, surface, &n, modes);
    int offersWanted = 0;
    int offersMailbox = 0;
    for (uint32_t i = 0; i < n; i++) {
        offersWanted |= modes[i] == wanted;
        offersMailbox |= modes[i] == VK_PRESENT_MODE_MAILBOX_KHR;
    }
    free(modes);
    if (offersWanted) {
        return wanted;
    }
    if (wanted == VK_PRESENT_MODE_IMMEDIATE_KHR && offersMailbox) {
        return VK_PRESENT_MODE_MAILBOX_KHR;
    }
    return VK_PRESENT_MODE_FIFO_KHR;
}

// vkbCreateSwapchain creates (or recreates from old) a swapchain of the given format,
// in the sRGB colour space — the caller picks it from vkbSurfaceFormats — presenting in
// presentMode, or the nearest the surface offers (see vkbPresentMode). Returns the
// swapchain and extent.
 VkResult vkbCreateSwapchain(VkPhysicalDevice phys, VkDevice dev, VkSurfaceKHR surface,
                                   uint32_t w, uint32_t h, VkFormat format, VkPresentModeKHR presentMode, VkSwapchainKHR old,
                                   VkSwapchainKHR* outSwap, uint32_t* outW, uint32_t* outH) {
    VkSurfaceCapabilitiesKHR caps;
    vkGetPhysicalDeviceSurfaceCapabilitiesKHR(phys, surface, &caps);

    VkExtent2D ext = caps.currentExtent;
    if (ext.width == 0xFFFFFFFF) {
        ext.width = w;
        ext.height = h;
    }

    uint32_t minImg = caps.minImageCount + 1;
    if (caps.maxImageCount > 0 && minImg > caps.maxImageCount) {
        minImg = caps.maxImageCount;
    }

    VkSwapchainCreateInfoKHR ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_SWAPCHAIN_CREATE_INFO_KHR;
    ci.surface = surface;
    ci.minImageCount = minImg;
    ci.imageFormat = format;
    ci.imageColorSpace = VK_COLOR_SPACE_SRGB_NONLINEAR_KHR;
    ci.imageExtent = ext;
    ci.imageArrayLayers = 1;
    // TRANSFER_SRC lets the frame be copied back out (screenshots). It is optional:
    // the spec only guarantees COLOR_ATTACHMENT, so ask for it only where the surface
    // reports it, and a driver that refuses simply means no screenshots rather than a
    // failed swapchain.
    ci.imageUsage = VK_IMAGE_USAGE_COLOR_ATTACHMENT_BIT | VK_IMAGE_USAGE_TRANSFER_DST_BIT;
    if (caps.supportedUsageFlags & VK_IMAGE_USAGE_TRANSFER_SRC_BIT) {
        ci.imageUsage |= VK_IMAGE_USAGE_TRANSFER_SRC_BIT;
    }
    ci.imageSharingMode = VK_SHARING_MODE_EXCLUSIVE;
    ci.preTransform = caps.currentTransform;
    ci.compositeAlpha = VK_COMPOSITE_ALPHA_OPAQUE_BIT_KHR;
    ci.presentMode = vkbPresentMode(phys, surface, presentMode);
    ci.clipped = VK_TRUE;
    ci.oldSwapchain = old;

    VkResult r = vkCreateSwapchainKHR(dev, &ci, NULL, outSwap);
    *outW = ext.width; *outH = ext.height;
    return r;
}

 uint32_t vkbSwapchainImageCount(VkDevice dev, VkSwapchainKHR swap) {
    uint32_t n = 0;
    vkGetSwapchainImagesKHR(dev, swap, &n, NULL);
    return n;
}

 void vkbSwapchainImages(VkDevice dev, VkSwapchainKHR swap, uint32_t n, VkImage* out) {
    vkGetSwapchainImagesKHR(dev, swap, &n, out);
}

 VkResult vkbSwapImageView(VkDevice dev, VkImage img, VkFormat fmt, VkImageView* out) {
    VkImageViewCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO;
    ci.image = img;
    ci.viewType = VK_IMAGE_VIEW_TYPE_2D;
    ci.format = fmt;
    ci.subresourceRange.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT;
    ci.subresourceRange.levelCount = 1;
    ci.subresourceRange.layerCount = 1;
    return vkCreateImageView(dev, &ci, NULL, out);
}

 VkResult vkbCreateSemaphore(VkDevice dev, VkSemaphore* out) {
    VkSemaphoreCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO;
    return vkCreateSemaphore(dev, &ci, NULL, out);
}

 VkResult vkbAcquire(VkDevice dev, VkSwapchainKHR swap, VkSemaphore sem, uint32_t* outIndex) {
    return vkAcquireNextImageKHR(dev, swap, UINT64_MAX, sem, VK_NULL_HANDLE, outIndex);
}

// vkbPresentBarrier transitions a backbuffer from GENERAL to PRESENT_SRC — the one
// layout unified image layouts leave in place, since the presentation engine reads it.
// Anything may have written the backbuffer last: a draw, a copy.
 void vkbPresentBarrier(VkCommandBuffer cb, VkImage img) {
    VkImageMemoryBarrier2 ib = {0};
    ib.sType = VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER_2;
    ib.srcStageMask = VK_PIPELINE_STAGE_2_ALL_COMMANDS_BIT;
    ib.srcAccessMask = VK_ACCESS_2_MEMORY_WRITE_BIT;
    ib.dstStageMask = VK_PIPELINE_STAGE_2_BOTTOM_OF_PIPE_BIT;
    ib.oldLayout = VK_IMAGE_LAYOUT_GENERAL;
    ib.newLayout = VK_IMAGE_LAYOUT_PRESENT_SRC_KHR;
    ib.image = img;
    ib.subresourceRange.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT;
    ib.subresourceRange.levelCount = VK_REMAINING_MIP_LEVELS;
    ib.subresourceRange.layerCount = VK_REMAINING_ARRAY_LAYERS;
    VkDependencyInfo di = {0};
    di.sType = VK_STRUCTURE_TYPE_DEPENDENCY_INFO;
    di.imageMemoryBarrierCount = 1;
    di.pImageMemoryBarriers = &ib;
    vkCmdPipelineBarrier2(cb, &di);
}

// vkbSubmitPresent submits cb (waiting on acquire, signalling renderDone) then
// presents the image. Bundled so the semaphore wiring stays in C.
 VkResult vkbSubmitPresent(VkQueue q, VkCommandBuffer cb, VkSwapchainKHR swap, uint32_t imageIndex,
                                 VkSemaphore acquire, VkSemaphore renderDone, VkFence fence) {
    VkSemaphoreSubmitInfo wait = {0};
    wait.sType = VK_STRUCTURE_TYPE_SEMAPHORE_SUBMIT_INFO;
    wait.semaphore = acquire;
    wait.stageMask = VK_PIPELINE_STAGE_2_COLOR_ATTACHMENT_OUTPUT_BIT;

    VkSemaphoreSubmitInfo sig = {0};
    sig.sType = VK_STRUCTURE_TYPE_SEMAPHORE_SUBMIT_INFO;
    sig.semaphore = renderDone;
    sig.stageMask = VK_PIPELINE_STAGE_2_ALL_COMMANDS_BIT;

    VkCommandBufferSubmitInfo cbi = {0};
    cbi.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_SUBMIT_INFO;
    cbi.commandBuffer = cb;

    VkSubmitInfo2 si = {0};
    si.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO_2;
    si.waitSemaphoreInfoCount = 1;   si.pWaitSemaphoreInfos = &wait;
    si.commandBufferInfoCount = 1;   si.pCommandBufferInfos = &cbi;
    si.signalSemaphoreInfoCount = 1; si.pSignalSemaphoreInfos = &sig;
    VkResult r = vkQueueSubmit2(q, 1, &si, fence);
    if (r != VK_SUCCESS) return r;

    VkPresentInfoKHR pi = {0};
    pi.sType = VK_STRUCTURE_TYPE_PRESENT_INFO_KHR;
    pi.waitSemaphoreCount = 1;
    pi.pWaitSemaphores = &renderDone;
    pi.swapchainCount = 1;
    pi.pSwapchains = &swap;
    pi.pImageIndices = &imageIndex;
    return vkQueuePresentKHR(q, &pi);
}

// Images and image views.
// vkbCreateImage creates an image + dedicated device-local memory + a default
// view. usage is a VkImageUsageFlags mask assembled on the Go side.
 VkResult vkbCreateImage(VkDevice dev, VkPhysicalDevice phys,
                               VkFormat fmt, uint32_t w, uint32_t h, uint32_t depth, uint32_t layers, uint32_t mips, uint32_t samples,
                               VkImageUsageFlags usage, VkImageAspectFlags aspect, VkImageViewType viewType,
                               VkImage* outImg, VkDeviceMemory* outMem, VkImageView* outView) {
    VkImageCreateInfo ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_IMAGE_CREATE_INFO;
    ci.imageType = viewType == VK_IMAGE_VIEW_TYPE_3D ? VK_IMAGE_TYPE_3D : VK_IMAGE_TYPE_2D;
    ci.format = fmt;
    ci.extent.width = w; ci.extent.height = h; ci.extent.depth = depth;
    ci.mipLevels = mips;
    ci.arrayLayers = layers;
    ci.samples = samples < 2 ? VK_SAMPLE_COUNT_1_BIT : (VkSampleCountFlagBits)samples;
    ci.tiling = VK_IMAGE_TILING_OPTIMAL;
    ci.usage = usage;
    ci.sharingMode = VK_SHARING_MODE_EXCLUSIVE;
    ci.initialLayout = VK_IMAGE_LAYOUT_UNDEFINED;
    if (viewType == VK_IMAGE_VIEW_TYPE_CUBE || viewType == VK_IMAGE_VIEW_TYPE_CUBE_ARRAY)
        ci.flags |= VK_IMAGE_CREATE_CUBE_COMPATIBLE_BIT;

    VkResult r = vkCreateImage(dev, &ci, NULL, outImg);
    if (r != VK_SUCCESS) return r;

    VkMemoryRequirements req;
    vkGetImageMemoryRequirements(dev, *outImg, &req);
    VkPhysicalDeviceMemoryProperties mp;
    vkGetPhysicalDeviceMemoryProperties(phys, &mp);
    uint32_t ti = 0xFFFFFFFF;
    // A transient attachment takes lazily allocated memory where the device has it —
    // tile-based GPUs, which then never commit any — and device-local memory otherwise.
    if (usage & VK_IMAGE_USAGE_TRANSIENT_ATTACHMENT_BIT) {
        for (uint32_t i = 0; i < mp.memoryTypeCount; i++) {
            if ((req.memoryTypeBits & (1u << i)) &&
                (mp.memoryTypes[i].propertyFlags & VK_MEMORY_PROPERTY_LAZILY_ALLOCATED_BIT)) { ti = i; break; }
        }
    }
    for (uint32_t i = 0; ti == 0xFFFFFFFF && i < mp.memoryTypeCount; i++) {
        if ((req.memoryTypeBits & (1u << i)) &&
            (mp.memoryTypes[i].propertyFlags & VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT)) { ti = i; break; }
    }
    if (ti == 0xFFFFFFFF) { vkDestroyImage(dev, *outImg, NULL); return VK_ERROR_OUT_OF_DEVICE_MEMORY; }

    VkMemoryAllocateInfo ai = {0};
    ai.sType = VK_STRUCTURE_TYPE_MEMORY_ALLOCATE_INFO;
    ai.allocationSize = req.size;
    ai.memoryTypeIndex = ti;
    r = vkAllocateMemory(dev, &ai, NULL, outMem);
    if (r != VK_SUCCESS) { vkDestroyImage(dev, *outImg, NULL); return r; }
    r = vkBindImageMemory(dev, *outImg, *outMem, 0);
    if (r != VK_SUCCESS) { vkFreeMemory(dev, *outMem, NULL); vkDestroyImage(dev, *outImg, NULL); return r; }

    VkImageViewCreateInfo vci = {0};
    vci.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO;
    vci.image = *outImg;
    vci.viewType = viewType;
    vci.format = fmt;
    vci.subresourceRange.aspectMask = aspect;
    vci.subresourceRange.levelCount = mips;
    vci.subresourceRange.layerCount = layers;
    r = vkCreateImageView(dev, &vci, NULL, outView);
    if (r != VK_SUCCESS) { vkFreeMemory(dev, *outMem, NULL); vkDestroyImage(dev, *outImg, NULL); return r; }
    return VK_SUCCESS;
}

// vkbWriteSampledImage registers a view into the bindless sampled-image array. The
// image is in VK_IMAGE_LAYOUT_GENERAL whenever the descriptor is used, as every image
// is — including a depth buffer bound as an attachment in the same pass.
 void vkbWriteSampledImage(VkDevice dev, VkDescriptorSet set, uint32_t index, VkImageView view) {
    VkDescriptorImageInfo ii = {0};
    ii.imageView = view;
    ii.imageLayout = VK_IMAGE_LAYOUT_GENERAL;
    VkWriteDescriptorSet w = {0};
    w.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    w.dstSet = set;
    w.dstBinding = 0; // BIND_SAMPLED
    w.dstArrayElement = index;
    w.descriptorCount = 1;
    w.descriptorType = VK_DESCRIPTOR_TYPE_SAMPLED_IMAGE;
    w.pImageInfo = &ii;
    vkUpdateDescriptorSets(dev, 1, &w, 0, NULL);
}

 void vkbWriteStorageImage(VkDevice dev, VkDescriptorSet set, uint32_t index, VkImageView view) {
    VkDescriptorImageInfo ii = {0};
    ii.imageView = view;
    ii.imageLayout = VK_IMAGE_LAYOUT_GENERAL;
    VkWriteDescriptorSet w = {0};
    w.sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    w.dstSet = set;
    w.dstBinding = 1; // BIND_STORAGE
    w.dstArrayElement = index;
    w.descriptorCount = 1;
    w.descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_IMAGE;
    w.pImageInfo = &ii;
    vkUpdateDescriptorSets(dev, 1, &w, 0, NULL);
}

// vkbCreateSubView makes an additional view over a subresource range of an
// existing image (e.g. one array layer or mip), for registering into the heap.
 VkResult vkbCreateSubView(VkDevice dev, VkImage img, VkFormat fmt, VkImageAspectFlags aspect,
                                 VkImageViewType viewType, uint32_t baseMip, uint32_t mipCount,
                                 uint32_t baseLayer, uint32_t layerCount, VkImageView* outView) {
    VkImageViewCreateInfo vci = {0};
    vci.sType = VK_STRUCTURE_TYPE_IMAGE_VIEW_CREATE_INFO;
    vci.image = img;
    vci.viewType = viewType;
    vci.format = fmt;
    vci.subresourceRange.aspectMask = aspect;
    vci.subresourceRange.baseMipLevel = baseMip;
    vci.subresourceRange.levelCount = mipCount;
    vci.subresourceRange.baseArrayLayer = baseLayer;
    vci.subresourceRange.layerCount = layerCount;
    return vkCreateImageView(dev, &vci, NULL, outView);
}

 void vkbDestroyImage(VkDevice dev, VkImage img, VkDeviceMemory mem, VkImageView view) {
    vkDestroyImageView(dev, view, NULL);
    vkFreeMemory(dev, mem, NULL);
    vkDestroyImage(dev, img, NULL);
}
