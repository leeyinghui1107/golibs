// Copyright (c) 2014 The cefcapi authors. All rights reserved.
// License: BSD 3-clause.
// Website: https://github.com/CzarekTomczak/cefcapi

#include "include/capi/cef_base_capi.h"
#include <stdio.h>
#include <string.h>

///
// Increment the reference count.
///
void CEF_CALLBACK add_ref(cef_base_t* self) {
    go_AddRef((void *) self);
}

///
// Decrement the reference count.  Delete this object when no references
// remain.
///
int CEF_CALLBACK release(cef_base_t* self) {
    return go_Release((void *) self);
}

///
// Returns true (1) if the current reference count is 1.
///
int CEF_CALLBACK has_one_ref(cef_base_t* self) {
    return go_HasOneRef((void *) self);
}


void initialize_cef_base(cef_base_t* base) {
    size_t size = base->size;
    if (size <= 0) {
        printf("FATAL: initialize_cef_base failed, size member not set\n");
        return;
    }
    base->add_ref = add_ref;
    base->release = release;
    base->has_one_ref = has_one_ref;
    go_AddRef((void *) base);
}

