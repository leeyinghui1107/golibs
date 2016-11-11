package serial

import (
	"fmt"
)

/*
#include <stdio.h>
#include <stdbool.h>
#include <windows.h>
#include <winbase.h>

bool com_port_exists(int port)
{
    char buf[7];
    COMMCONFIG cfg;
    DWORD size;

	if (port < 1) return false;
	if (port > 255) return false;

    snprintf(buf, sizeof(buf), "COM%d", port);
    size = sizeof(cfg);

    // COM port exists if GetDefaultCommConfig returns TRUE
    if (GetDefaultCommConfig(buf, &cfg, &size)) return true;

	// changes <size> to indicate COMMCONFIG buffer too small.
    if (size > sizeof(cfg)) return true;

	return false;
}
*/
import "C"

func GetComPortList() []string {
	total := 0
	b := make([]byte, 20)
	for index := 0; index < cap(b); index++ {
		if C.com_port_exists(C.int(index + 1)) {
			b[index] = 1
			total++
		} else {
			b[index] = 0
		}
	}

	list := []string{}

	for index, v := range b {
		if v != 0 {
			list = append(list, fmt.Sprintf("COM%d", index+1))
		}
	}
	return list
}
