package cef

/*
CEF capi fixes
--------------
1. In cef_string.h:
    this => typedef cef_string_utf16_t cef_string_t;
    to => #define cef_string_t cef_string_utf16_t
2. In cef_export.h:
    #elif defined(COMPILER_GCC)
    #define CEF_EXPORT __attribute__ ((visibility("default")))
    #ifdef OS_WIN
    #define CEF_CALLBACK __stdcall
    #else
    #define CEF_CALLBACK
    #endif
*/

/*
#cgo CFLAGS: -I./cef_binary
#cgo LDFLAGS: -L./cef_binary/Release -lcef
#include <stdlib.h>
#include "string.h"
#include "cef_app.h"
#include "cef_client.h"
#include "cef_helpers.h"
*/
import "C"
import (
	"os"
	"time"
	"unsafe"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("cef")
var contextInitialized = make(chan int, 1)

var _MainArgs *C.struct__cef_main_args_t
var _AppHandler *C.cef_app_t               // requires reference counting
var _ClientHandler *C.struct__cef_client_t // requires reference counting

// Set up the js console handlers
type ConsoleHandlerFunc func(message, source string, line int)

var DefaultConsoleHandler ConsoleHandlerFunc = ConsoleHandlerFunc(func(message, source string, line int) {
	log.Info("[console:%s %d] %s", source, line, message)
})
var consoleHandler ConsoleHandlerFunc = DefaultConsoleHandler

// Sandbox is disabled. Including the "cef_sandbox.lib"
// library results in lots of GCC warnings/errors. It is
// compatible only with VS 2010. It would be required to
// build it using GCC. Add -lcef_sandbox to LDFLAGS.
// capi doesn't expose sandbox functions, you need do add
// these before import "C":
// void* cef_sandbox_info_create();
// void cef_sandbox_info_destroy(void* sandbox_info);
var _SandboxInfo unsafe.Pointer

type Settings struct {
	SingleProcess       int
	CachePath           string
	LogSeverity         int
	LogFile             string
	ResourcesDirPath    string
	LocalesDirPath      string
	JavaScriptFlags     string
	RemoteDebuggingPort int
}

const (
	LOGSEVERITY_DEFAULT = C.LOGSEVERITY_DEFAULT
	LOGSEVERITY_VERBOSE = C.LOGSEVERITY_VERBOSE
	LOGSEVERITY_INFO    = C.LOGSEVERITY_INFO
	LOGSEVERITY_WARNING = C.LOGSEVERITY_WARNING
	LOGSEVERITY_ERROR   = C.LOGSEVERITY_ERROR
	LOGSEVERITY_DISABLE = C.LOGSEVERITY_DISABLE
)

func CEFString(original string) (final *C.cef_string_t) {
	final = (*C.cef_string_t)(C.calloc(1, C.sizeof_cef_string_t))
	charString := C.CString(original)
	defer C.free(unsafe.Pointer(charString))
	C.cef_string_from_utf8(charString, C.strlen(charString), final)
	return final
}

func CEFToGoString(source *C.cef_string_t) string {
	utf8string := C.cefSourceToString(source)
	defer C.cef_string_userfree_utf8_free(utf8string)
	return C.GoString(utf8string.str)
}

func _InitializeGlobalCStructures() {
	_MainArgs = (*C.struct__cef_main_args_t)(C.calloc(1, C.sizeof_struct__cef_main_args_t))
	go_AddRef(unsafe.Pointer(_MainArgs))

	_AppHandler = (*C.cef_app_t)(C.calloc(1, C.sizeof_cef_app_t))
	go_AddRef(unsafe.Pointer(_AppHandler))
	C.initialize_app_handler(_AppHandler)

	_ClientHandler = (*C.struct__cef_client_t)(C.calloc(1, C.sizeof_struct__cef_client_t))
	go_AddRef(unsafe.Pointer(_ClientHandler))
	C.initialize_client_handler(_ClientHandler)
}

func ExecuteProcess(appHandle unsafe.Pointer) int {
	log.Debug("ExecuteProcess, args=%v", os.Args)

	_InitializeGlobalCStructures()
	FillMainArgs(_MainArgs, appHandle)

	// Sandbox info needs to be passed to both cef_execute_process()
	// and cef_initialize().
	// OFF: _SandboxInfo = C.cef_sandbox_info_create()

	var exitCode C.int = C.cef_execute_process(_MainArgs, _AppHandler, nil)
	if exitCode >= 0 {
		os.Exit(int(exitCode))
	}
	log.Debug("Finished ExecuteProcess, args=%v %d %d", os.Args, os.Getpid(), exitCode)
	return int(exitCode)
}

func cefStateFromBool(state bool) C.cef_state_t {
	if state == true {
		return C.STATE_ENABLED
	} else {
		return C.STATE_DISABLED
	}
}

func (settings *Settings) ToCStruct() (cefSettings *C.struct__cef_settings_t) {
	// Initialize cef_settings_t structure.
	cefSettings = (*C.struct__cef_settings_t)(C.calloc(1, C.sizeof_struct__cef_settings_t))
	cefSettings.size = C.sizeof_struct__cef_settings_t
	cefSettings.single_process = C.int(settings.SingleProcess)
	cefSettings.cache_path = *CEFString(settings.CachePath)
	cefSettings.log_severity = (C.cef_log_severity_t)(C.int(settings.LogSeverity))
	cefSettings.log_file = *CEFString(settings.LogFile)
	cefSettings.resources_dir_path = *CEFString(settings.ResourcesDirPath)
	cefSettings.locales_dir_path = *CEFString(settings.LocalesDirPath)
	cefSettings.remote_debugging_port = C.int(settings.RemoteDebuggingPort)
	cefSettings.javascript_flags = *CEFString(settings.JavaScriptFlags)

	cefSettings.no_sandbox = C.int(1)
	return
}

func Initialize(settings Settings) int {
	log.Debug("Initialize")

	if _MainArgs == nil {
		// _MainArgs structure is initialized and filled in ExecuteProcess.
		// If cef_execute_process is not called, and there is a call
		// to cef_initialize, then it would result in creation of infinite
		// number of processes. See Issue 1199 in CEF:
		// https://code.google.com/p/chromiumembedded/issues/detail?id=1199
		log.Error("ERROR: missing a call to ExecuteProcess")
		return 0
	}

	globalLifespanHandler = &LifeSpanHandler{make(chan *Browser)}
	go_AddRef(unsafe.Pointer(_AppHandler))
	ret := C.cef_initialize(_MainArgs, settings.ToCStruct(), _AppHandler, nil)
	log.Debug("cef_initalize: %d", int(ret))
	if OnUIThread() == true {
		WaitForContextInitialized()
	}
	// Sleep for 1500ms to let cef _really_ initialize
	// https://code.google.com/p/cefpython/issues/detail?id=131#c2
	// time.Sleep(2500 * time.Millisecond)

	return int(ret)
}

func WaitForContextInitialized() {
	select {
	case <-contextInitialized:
		return
	case <-time.After(10 * time.Second):
		log.Error("Timed out waiting for OnContextInitialized")
	}
}

func RunMessageLoop() {
	log.Debug("RunMessageLoop")
	C.cef_run_message_loop()
	time.Sleep(1 * time.Second)
}

func QuitMessageLoop() {
	log.Debug("QuitMessageLoop")
	C.cef_quit_message_loop()
}

func Shutdown() {
	log.Debug("Shutdown")
	C.cef_shutdown()
}

func SetConsoleHandler(handler ConsoleHandlerFunc) {
	consoleHandler = handler
}

func OnUIThread() bool {
	return C.cef_currently_on(C.TID_UI) == 1
}
