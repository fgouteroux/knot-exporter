package libknot

/*
#define _GNU_SOURCE
#define _DEFAULT_SOURCE
#include <libknot/libknot.h>
#include <libknot/control/control.h>
#include <libknot/version.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#cgo CFLAGS: -std=c99
#cgo LDFLAGS: -L/usr/lib64 -lknot
#cgo pkg-config: libknot

// Get libknot version
const char* get_libknot_version() {
    static char version_buf[32];

    // Use the actual macros from version.h
    #if defined(KNOT_VERSION_MAJOR) && defined(KNOT_VERSION_MINOR) && defined(KNOT_VERSION_PATCH)
        // Handle KNOT_VERSION_PATCH which might be hex (like 0x012)
        int patch = KNOT_VERSION_PATCH;
        // Convert from hex to decimal if needed (0x012 -> 18)
        if (patch > 99) {
            patch = ((patch >> 4) & 0xF) * 10 + (patch & 0xF);
        }
        snprintf(version_buf, sizeof(version_buf), "%d.%d.%d",
                 KNOT_VERSION_MAJOR, KNOT_VERSION_MINOR, patch);
        return version_buf;
    #elif defined(KNOT_VERSION_HEX)
        // Fallback to hex version if individual components not available
        int major = (KNOT_VERSION_HEX >> 16) & 0xFF;
        int minor = (KNOT_VERSION_HEX >> 8) & 0xFF;
        int patch = KNOT_VERSION_HEX & 0xFF;
        // Convert patch from hex to decimal if it looks like BCD
        if (patch > 99) {
            patch = ((patch >> 4) & 0xF) * 10 + (patch & 0xF);
        }
        snprintf(version_buf, sizeof(version_buf), "%d.%d.%d", major, minor, patch);
        return version_buf;
    #else
        return "unknown";
    #endif
}

// Wrapper functions for libknot control interface
knot_ctl_t* knot_ctl_alloc_wrapper() {
    return knot_ctl_alloc();
}

void knot_ctl_free_wrapper(knot_ctl_t *ctl) {
    knot_ctl_free(ctl);
}

int knot_ctl_connect_wrapper(knot_ctl_t *ctl, const char *path) {
    return knot_ctl_connect(ctl, path);
}

void knot_ctl_close_wrapper(knot_ctl_t *ctl) {
    knot_ctl_send(ctl, KNOT_CTL_TYPE_END, NULL);
    knot_ctl_close(ctl);
}

void knot_ctl_set_timeout_wrapper(knot_ctl_t *ctl, int timeout_ms) {
    knot_ctl_set_timeout(ctl, timeout_ms);
}

// Send a command with record type
int send_command_with_type(knot_ctl_t *ctl, const char *cmd, const char *rtype) {
    knot_ctl_data_t data;
    memset(data, 0, sizeof(data));

    data[KNOT_CTL_IDX_CMD] = cmd;
    if (rtype && strlen(rtype) > 0) {
        data[KNOT_CTL_IDX_TYPE] = rtype;
    }

    int ret = knot_ctl_send(ctl, KNOT_CTL_TYPE_DATA, &data);
    if (ret != 0) return ret;

    return knot_ctl_send(ctl, KNOT_CTL_TYPE_BLOCK, NULL);
}

// Receive response and extract key fields
int receive_simple_response(knot_ctl_t *ctl, knot_ctl_type_t *type,
                           char *section, char *id, char *item, char *zone, char *data_value,
                           int section_size, int id_size, int item_size, int zone_size, int data_size) {
    knot_ctl_data_t data;
    memset(data, 0, sizeof(data));

    int ret = knot_ctl_receive(ctl, type, &data);
    if (ret != 0) return ret;

    // Copy strings safely
    if (section && data[KNOT_CTL_IDX_SECTION]) {
        strncpy(section, data[KNOT_CTL_IDX_SECTION], section_size - 1);
        section[section_size - 1] = '\0';
    } else if (section) {
        section[0] = '\0';
    }

    if (id && data[KNOT_CTL_IDX_ID]) {
        strncpy(id, data[KNOT_CTL_IDX_ID], id_size - 1);
        id[id_size - 1] = '\0';
    } else if (id) {
        id[0] = '\0';
    }

    if (item && data[KNOT_CTL_IDX_ITEM]) {
        strncpy(item, data[KNOT_CTL_IDX_ITEM], item_size - 1);
        item[item_size - 1] = '\0';
    } else if (item) {
        item[0] = '\0';
    }

    if (zone && data[KNOT_CTL_IDX_ZONE]) {
        strncpy(zone, data[KNOT_CTL_IDX_ZONE], zone_size - 1);
        zone[zone_size - 1] = '\0';
    } else if (zone) {
        zone[0] = '\0';
    }

    if (data_value && data[KNOT_CTL_IDX_DATA]) {
        strncpy(data_value, data[KNOT_CTL_IDX_DATA], data_size - 1);
        data_value[data_size - 1] = '\0';
    } else if (data_value) {
        data_value[0] = '\0';
    }

    return 0;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// CtlType defines the control data unit types
type CtlType int

const (
	CtlTypeEnd   CtlType = 0 // KNOT_CTL_TYPE_END
	CtlTypeData  CtlType = 1 // KNOT_CTL_TYPE_DATA
	CtlTypeExtra CtlType = 2 // KNOT_CTL_TYPE_EXTRA
	CtlTypeBlock CtlType = 3 // KNOT_CTL_TYPE_BLOCK
)

// CtlData holds response data from libknot control interface
type CtlData struct {
	Section string
	ID      string
	Item    string
	Zone    string
	Data    string
}

// CtlError defines custom error types for control operations
type CtlError struct {
	message string
	data    *CtlData
}

func (e *CtlError) Error() string {
	out := e.message
	if e.data != nil {
		out += fmt.Sprintf(" (%+v)", e.data)
	}
	return out
}

// Derived error types
type CtlErrorConnect struct{ CtlError }
type CtlErrorSend struct{ CtlError }
type CtlErrorReceive struct{ CtlError }
type CtlErrorRemote struct{ CtlError }

// Ctl manages interactions with the Knot DNS server control interface
type Ctl struct {
	ctl *C.knot_ctl_t
}

// New creates a new Knot control interface instance
func New() *Ctl {
	ctl := C.knot_ctl_alloc_wrapper()
	if ctl == nil {
		return nil
	}
	return &Ctl{ctl: ctl}
}

// Close closes the control interface and frees resources
func (k *Ctl) Close() {
	if k.ctl != nil {
		C.knot_ctl_close_wrapper(k.ctl)
		C.knot_ctl_free_wrapper(k.ctl)
		k.ctl = nil
	}
}

// SetTimeout sets the timeout for control operations
func (k *Ctl) SetTimeout(timeout int) {
	if k.ctl != nil {
		// Cast safely to C.int with bounds checking
		var cTimeout C.int
		if timeout > 0 && timeout <= 2147483647 { // Max value for int32/C.int
			cTimeout = C.int(timeout)
		} else if timeout <= 0 {
			cTimeout = 0 // Provide a safe default for negative values
		} else {
			cTimeout = 2147483647 // Use maximum allowed value if input is too large
		}
		C.knot_ctl_set_timeout_wrapper(k.ctl, cTimeout)
	}
}

// Connect connects to the Knot DNS control socket
func (k *Ctl) Connect(path string) error {
	if k.ctl == nil {
		return &CtlErrorConnect{CtlError{message: "control object not initialized"}}
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	ret := C.knot_ctl_connect_wrapper(k.ctl, cPath)
	if ret != 0 {
		err := C.GoString(C.knot_strerror(ret))
		return &CtlErrorConnect{CtlError{message: err}}
	}
	return nil
}

// SendCommand sends a command to the Knot DNS server
func (k *Ctl) SendCommand(cmd string) error {
	if k.ctl == nil {
		return &CtlErrorSend{CtlError{message: "control object not initialized"}}
	}

	cCmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(cCmd))

	ret := C.send_command_with_type(k.ctl, cCmd, nil)
	if ret != 0 {
		err := C.GoString(C.knot_strerror(ret))
		return &CtlErrorSend{CtlError{message: err}}
	}
	return nil
}

// SendCommandWithType sends a command with a specific record type to the Knot DNS server
func (k *Ctl) SendCommandWithType(cmd string, rtype string) error {
	if k.ctl == nil {
		return &CtlErrorSend{CtlError{message: "control object not initialized"}}
	}

	cCmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(cCmd))

	cType := C.CString(rtype)
	defer C.free(unsafe.Pointer(cType))

	ret := C.send_command_with_type(k.ctl, cCmd, cType)
	if ret != 0 {
		err := C.GoString(C.knot_strerror(ret))
		return &CtlErrorSend{CtlError{message: err}}
	}
	return nil
}

// ReceiveResponse receives a response from the Knot DNS server
func (k *Ctl) ReceiveResponse() (CtlType, *CtlData, error) {
	if k.ctl == nil {
		return 0, nil, &CtlErrorReceive{CtlError{message: "control object not initialized"}}
	}

	var dataType C.knot_ctl_type_t

	// Allocate buffers for the response
	const bufSize = 1024
	sectionBuf := make([]C.char, bufSize)
	idBuf := make([]C.char, bufSize)
	itemBuf := make([]C.char, bufSize)
	zoneBuf := make([]C.char, bufSize)
	dataBuf := make([]C.char, bufSize)

	ret := C.receive_simple_response(k.ctl, &dataType,
		&sectionBuf[0], &idBuf[0], &itemBuf[0], &zoneBuf[0], &dataBuf[0],
		C.int(bufSize), C.int(bufSize), C.int(bufSize), C.int(bufSize), C.int(bufSize))

	if ret != 0 {
		err := C.GoString(C.knot_strerror(ret))
		return 0, nil, &CtlErrorReceive{CtlError{message: err}}
	}

	data := &CtlData{
		Section: C.GoString(&sectionBuf[0]),
		ID:      C.GoString(&idBuf[0]),
		Item:    C.GoString(&itemBuf[0]),
		Zone:    C.GoString(&zoneBuf[0]),
		Data:    C.GoString(&dataBuf[0]),
	}

	return CtlType(dataType), data, nil
}

// GetVersion returns the libknot version
func GetVersion() string {
	return C.GoString(C.get_libknot_version())
}
