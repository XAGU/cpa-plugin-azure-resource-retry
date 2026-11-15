package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static cliproxy_host_api stored_host;

static void store_host(const cliproxy_host_api* host) {
	stored_host = *host;
}

static int call_host(const char* method, const uint8_t* request, size_t len, cliproxy_buffer* response) {
	if (stored_host.call == NULL) return 1;
	return stored_host.call(stored_host.host_ctx, method, request, len, response);
}

static void free_host(void* ptr, size_t len) {
	if (ptr != NULL && stored_host.free_buffer != NULL) stored_host.free_buffer(ptr, len);
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/XAGU/cpa-plugin-azure-resource-retry/internal/plugin"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
)

var handler = plugin.New(callHost)

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, api *C.cliproxy_plugin_api) C.int {
	if host == nil || api == nil || host.abi_version != C.uint32_t(pluginabi.ABIVersion) || host.call == nil || host.free_buffer == nil {
		return 1
	}
	C.store_host(host)
	api.abi_version = C.uint32_t(pluginabi.ABIVersion)
	api.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	api.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	api.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, length C.size_t, response *C.cliproxy_buffer) (status C.int) {
	if response == nil {
		return 1
	}
	response.ptr, response.len = nil, 0
	defer func() {
		if recover() != nil {
			raw, _ := pluginabi.NewErrorEnvelope("plugin_error", "插件处理请求时发生内部错误", 500)
			writeResponse(response, raw)
			status = 1
		}
	}()
	if method == nil || (request == nil && length != 0) || uint64(length) > uint64(^uint32(0)>>1) {
		raw, _ := pluginabi.NewErrorEnvelope("invalid_request", "无效的插件调用参数", 400)
		writeResponse(response, raw)
		return 1
	}
	raw, err := handler.Handle(C.GoString(method), C.GoBytes(unsafe.Pointer(request), C.int(length)))
	if err != nil {
		raw, _ = pluginabi.NewErrorEnvelope("plugin_error", err.Error(), 500)
		writeResponse(response, raw)
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	C.free(ptr)
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if len(raw) == 0 {
		return
	}
	response.ptr = C.CBytes(raw)
	response.len = C.size_t(len(raw))
}

func callHost(method string, raw []byte) ([]byte, error) {
	name := C.CString(method)
	defer C.free(unsafe.Pointer(name))
	request := C.CBytes(raw)
	defer C.free(request)
	var response C.cliproxy_buffer
	status := C.call_host(name, (*C.uint8_t)(request), C.size_t(len(raw)), &response)
	defer C.free_host(response.ptr, response.len)
	if response.ptr == nil || response.len == 0 || uint64(response.len) > uint64(^uint32(0)>>1) {
		return nil, fmt.Errorf("宿主回调 %s 未返回有效响应，状态码 %d", method, int(status))
	}
	return C.GoBytes(response.ptr, C.int(response.len)), nil
}
