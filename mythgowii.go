package main

/*
#cgo CFLAGS: -I/usr/include
#cgo LDFLAGS: -lcwiid -Lcwiid/libcwiid/libcwiid.a
#include "mythgowii.h"
#include <stdlib.h>
#include <cwiid.h>
#include <time.h>
#include <bluetooth/bluetooth.h>
*/
import "C"

import (
	"fmt"
	"log"
	"net/textproto"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

var buttons = []_Ctype_uint16_t{
	C.CWIID_BTN_A,
	C.CWIID_BTN_B,
	C.CWIID_BTN_1,
	C.CWIID_BTN_2,
	C.CWIID_BTN_MINUS,
	C.CWIID_BTN_HOME,
	C.CWIID_BTN_LEFT,
	C.CWIID_BTN_RIGHT,
	C.CWIID_BTN_DOWN,
	C.CWIID_BTN_UP,
	C.CWIID_BTN_PLUS,
}

const (
	Connected    int8 = iota
	Disconnected int8 = iota
	Error        int8 = iota
	Finished     int8 = iota
)

var buttonStatus []bool

var wiimoteStatus int8 // accessed only inside the goCwiidCallback and the connectWiimote functions
var tellWiimote chan bool
var mythChanOut chan string
var mythChanIn chan string
var buttonChan chan _Ctype_uint16_t

var callback = goCwiidCallback  // so it's not garbage collected
var errCallback = goErrCallback // so it's not garbage collected

func init() {
	mythChanOut = make(chan string)
	mythChanIn = make(chan string)
	tellWiimote = make(chan bool)
	wiimoteStatus = Disconnected
	buttonChan = make(chan _Ctype_uint16_t)
	buttonStatus = make([]bool, len(buttons))
}

//export goCwiidCallback
func goCwiidCallback(wm unsafe.Pointer, a int, mesg *C.struct_cwiid_btn_mesg, tp unsafe.Pointer) {
	//defer C.free(unsafe.Pointer(mesg))
	var messages []C.struct_cwiid_btn_mesg
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&messages)))
	sliceHeader.Cap = a
	sliceHeader.Len = a
	sliceHeader.Data = uintptr(unsafe.Pointer(mesg))
	//log.Printf("Got messages %v from wiimote", messages)
	for _, m := range messages {
		if m._type != C.CWIID_MESG_BTN {
			log.Printf("Got unexpected message %v from wiimote - %v", m)
			tellWiimote <- true
			continue
		}
		for x, button := range buttons {
			if m.buttons&button == button {
				if !buttonStatus[x] {
					buttonChan <- button
					buttonStatus[x] = true
				}
			} else {
				buttonStatus[x] = false
			}
		}
	}
}

//export goErrCallback
func goErrCallback(wm unsafe.Pointer, char *C.char, ap unsafe.Pointer) {
	//func goErrCallback(wm *C.cwiid_wiimote_t, char *C.char, ap C.va_list) {
	str := C.GoString(char)
	log.Printf("Found error %s in goErrCallback", str)
	switch str {
	case "No Bluetooth interface found":
		fallthrough
	case "no such device":
		log.Fatalf("No Bluetooth device found\n")
	case "Socket connect error (control socket)":
		fallthrough
	case "Socket connect error (interrupt socket)":
		fallthrough
	case "Socket connect error (control channel)":
		fallthrough
	case "No wiimotes found":
		wiimoteStatus = Disconnected
	default:
		log.Fatalf("Inside error callback - %s\n", str)
		wiimoteStatus = Error
	}
}

func readAll(conn *textproto.Conn, mythError chan error) {
	for {
		msg, err := conn.ReadLine()
		if err != nil {
			mythError <- fmt.Errorf("Error reading from myth - %v", err)
			return
		}
		// if we're not watching something, don't send any notification
		switch {
		case strings.HasPrefix(msg, "# Playback Recorded"):
			fallthrough
		case strings.HasPrefix(msg, "# Playback Video"):
			fallthrough
		case strings.HasPrefix(msg, "# Playback DVD"):
			mythChanIn <- msg
		}
	}
}

func connectMyth(conn *textproto.Conn, mythError chan error) (*textproto.Conn, error) {
	if conn != nil {
		return conn, nil
	}
	conn, err := textproto.Dial("tcp", "localhost:6546")
	if err != nil {
		log.Printf("Error connecting to mythfrontend - %v", err)
		return nil, err
	}
	log.Printf("Connected to Myth")
	mythChanIn <- "Connected"
	go readAll(conn, mythError)
	return conn, nil
}

func monitorMyth() {
	var conn *textproto.Conn
	var err error
	mythError := make(chan error)
	for {
		select {
		case err = <-mythError:
			conn = nil
			log.Printf("Got error reading from myth - %v")
		case msg := <-mythChanOut:
			conn, _ = connectMyth(conn, mythError)
			if conn == nil {
				log.Printf("Can't send message, not connected to mythfrontend - %s", msg)
			} else {
				_, err := conn.Cmd("%s", msg)
				if err != nil {
					log.Printf("Error writing to myth - %v", err)
					conn = nil
				}
			}
		case <-time.After(time.Second * 30):
			conn, _ = connectMyth(conn, mythError)
			if conn != nil {
				_, err = conn.Cmd("query location")
				if err != nil {
					log.Printf("Error getting status from myth - %v", err)
					conn = nil
				}
			}
		}
	}
}

func main() {
	go connectWiimote()
	go monitorMyth()
	for {
		select {
		case button := <-buttonChan:
			switch button {
			case C.CWIID_BTN_A:
				mythChanOut <- "key enter"
			case C.CWIID_BTN_B:
				mythChanOut <- "key z"
			case C.CWIID_BTN_1:
				mythChanOut <- "key i"
			case C.CWIID_BTN_2:
				mythChanOut <- "key m"
			case C.CWIID_BTN_MINUS:
				mythChanOut <- "key d"
			case C.CWIID_BTN_HOME:
				mythChanOut <- "key escape"
			case C.CWIID_BTN_LEFT:
				mythChanOut <- "key left"
			case C.CWIID_BTN_RIGHT:
				mythChanOut <- "key right"
			case C.CWIID_BTN_DOWN:
				mythChanOut <- "key down"
			case C.CWIID_BTN_UP:
				mythChanOut <- "key up"
			case C.CWIID_BTN_PLUS:
				mythChanOut <- "key p"
			}
		case msg := <-mythChanIn:
			log.Printf("Myth still connected, msg - %v", msg)
		case <-time.After(time.Minute):
			// nothing from either chan for a minute
			//log.Printf("Done watching, disconnecting wiimote")
			tellWiimote <- true
		}
	}
}

func connectWiimote() {
	var wm *C.struct_cwiid_wiimote_t
	val, err := C.cwiid_set_err(C.getErrCallback())
	if val != 0 || err != nil {
		log.Fatalf("Error setting the callback to catch errors - %d - %v", val, err)
	}
	for {
	outer:
		for {
			// clear the channels for any previous connection
			select {
			case <-tellWiimote:
			case <-buttonChan:
			default:
				break outer
			}
		}
		wiimoteStatus = Disconnected
		log.Println("Press 1&2 on the Wiimote now")
		wm, err = C.cwiid_open_timeout(nil, 0, 10)
		if err != nil {
			log.Printf("cwiid_open: %v, sleeping...\n", err)
			time.Sleep(time.Second)
			continue
		}
		if wm == nil {
			continue // could not connect to wiimote
		}
		log.Printf("Wiimote connected")
		wiimoteStatus = Connected
		res, err := C.cwiid_command(wm, C.CWIID_CMD_RPT_MODE, C.CWIID_RPT_BTN)
		if res != 0 || err != nil || wiimoteStatus != Connected {
			log.Printf("Result of command = %d - %v\n", res, err)
			continue
		}
		res, err = C.cwiid_set_mesg_callback(wm, C.getCwiidCallback())
		if res != 0 || err != nil || wiimoteStatus != Connected {
			log.Printf("Result of callback = %d - %v\n", res, err)
			continue
		}
		res, err = C.cwiid_enable(wm, C.CWIID_FLAG_MESG_IFC)
		if res != 0 || err != nil || wiimoteStatus != Connected {
			log.Printf("Result of enable = %d - %v\n", res, err)
			continue
		}
		res = C.cwiid_set_led(wm, C.CWIID_LED2_ON|C.CWIID_LED3_ON)
		if res != 0 || wiimoteStatus != Connected {
			log.Printf("Set led result = %d\n", res)
			continue
		}
		res = C.cwiid_set_rumble(wm, 1)
		if res != 0 || wiimoteStatus != Connected {
			log.Printf("Unable to set rumble mode")
			continue
		}
		time.Sleep(time.Millisecond * 200)
		res = C.cwiid_set_rumble(wm, 0)
		if res != 0 || wiimoteStatus != Connected {
			log.Printf("Unable to unset rumble mode")
			continue
		}
	loop:
		for {
			select {
			case <-tellWiimote:
				log.Printf("Being told to disconnect the wiimote")
				if wm != nil {
					log.Printf("Asking wiimote to be closed")
					res, err := C.cwiid_close(wm)
					if res != 0 || err != nil {
						log.Printf("Unable to close wiimote")
						continue
					}
					log.Printf("Closed wiimote")
				}
				wiimoteStatus = Disconnected
				wm = nil
				break loop // this takes us to the large loop above so that the wiimote can reconnect
			}
		}
	}
}
