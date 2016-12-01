// Copyright (c) 2014 The cef2go authors. All rights reserved.
// License: BSD 3-clause.
// Website: https://github.com/CzarekTomczak/cef2go

package cef

/*
#cgo CFLAGS: -I./cef_binary
#cgo LDFLAGS: -L./cefb_inary/Release -lcef
#include <stdlib.h>
#include "string.h"
#include "include/capi/cef_app_capi.h"
*/
import "C"
import "unsafe"

import (
	"os"
)

func FillMainArgs(mainArgs *C.struct__cef_main_args_t, appHandle unsafe.Pointer) {
	var _Argv []*C.char = make([]*C.char, len(os.Args))
	// On Linux appHandle is nil.
	log.Debug("FillMainArgs, argc=%d", len(os.Args))
	for i, arg := range os.Args {
		_Argv[C.int(i)] = C.CString(arg)
	}
	mainArgs.argc = C.int(len(os.Args))
	mainArgs.argv = &_Argv[0]
}

func FillWindowInfo(windowInfo *C.cef_window_info_t, hwnd unsafe.Pointer) {
	log.Debug("FillWindowInfo")
	//windowInfo.parent_widget = (*C.GtkWidget)(hwnd)
}
