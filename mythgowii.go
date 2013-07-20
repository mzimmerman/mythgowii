package main

/*
#cgo CFLAGS: -I/usr/include
#cgo LDFLAGS: -lcwiid -L/usr/lib/libcwiid.a
#include "mythgowii.h"
#include <stdlib.h>
#include <cwiid.h>
#include <time.h>
#include <bluetooth/bluetooth.h>
*/
import "C"

import (
	"fmt"
	"os"
	"time"
	"unsafe"
)

//export goCwiidCallback
func goCwiidCallback(wm unsafe.Pointer, a int, mesg []int, tp unsafe.Pointer) {
	//func goCwiidCallback(wm *C.cwiid_wiimote_t, a int, mesg []int, tp *C.struct_timespec) {
	fmt.Printf("inside the callback!\n")
	//wiimote := *C.cwiid_wiimote_t(wm)
	//t := *C.struct_timespec(&tp)
	fmt.Printf("made the callback - %v - %d - %v, %v", wm, a, mesg, tp)
	hold <- true
}

//export goErrCallback
func goErrCallback(wm unsafe.Pointer, char *C.char, ap unsafe.Pointer) {
	//func goErrCallback(wm *C.cwiid_wiimote_t, char *C.char, ap C.va_list) {
	str := C.GoString(char)
	switch str {
	case "No Bluetooth interface found":
		fallthrough
	case "no such device":
		fmt.Printf("No Bluetooth device found\n")
		os.Exit(1)
	default:
		fmt.Printf("Inside error calback - %s\n", str)
	}
}

var hold chan bool

func main() {
	hold = make(chan bool)
	var bdaddr C.bdaddr_t
	var wm *C.struct_cwiid_wiimote_t
	val, err := C.cwiid_set_err(C.getErrCallback())
	if val != 0 || err != nil {
		fmt.Printf("Error setting the callback to catch errors - %d - %v", val, err)
		os.Exit(1)
	}
	for {
		fmt.Println("Press 1&2 on the Wiimote now")
		wm, err = C.cwiid_open(&bdaddr, 0)
		if err != nil {
			char := C.CString("no such device")
			defer C.free(unsafe.Pointer(char))
			fmt.Printf("Error is %v", err)
			goErrCallback(nil, char, nil)
		}
		if wm != nil {
			break
		}
	}
	fmt.Printf("We got a wiimote!\n")
	led, err := C.cwiid_set_led(wm, C.CWIID_LED2_ON)
	if err != nil {
		fmt.Errorf("Err = %v", err)
	}
	fmt.Printf("Set led result = %d\n", led)
	//res := C.cwiid_set_mesg_callback(wm, (*C.cwiid_mesg_callback_t)(unsafe.Pointer(cb)))
	res, err := C.cwiid_set_mesg_callback(wm, C.getCwiidCallback())
	fmt.Printf("Result of callback = %d - %v\n", res, err)
	go func() {
		rumble := 0
		for x := 0; ; x++ {
			C.cwiid_set_rumble(wm, C.uint8_t(rumble))
			time.Sleep(time.Second)
			if rumble == 0 {
				rumble = 1
			} else {
				rumble = 0
			}
		}
	}()
	<-hold
}
