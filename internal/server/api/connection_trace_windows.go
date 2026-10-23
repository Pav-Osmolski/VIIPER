//go:build windows

package api

import (
	"fmt"
	"golang.org/x/sys/windows"
	"net"
	"unsafe"
)

func socketLinger(conn net.Conn) (bool, int, error) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return false, 0, fmt.Errorf("not TCP")
	}
	raw, err := tcp.SyscallConn()
	if err != nil {
		return false, 0, err
	}
	// Winsock's native linger has two uint16 fields; windows.Linger is the
	// portable setter input and deliberately has a different layout.
	var linger struct{ Onoff, Linger uint16 }
	size := int32(unsafe.Sizeof(linger))
	var socketErr error
	err = raw.Control(func(fd uintptr) {
		socketErr = windows.Getsockopt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_LINGER, (*byte)(unsafe.Pointer(&linger)), &size)
	})
	if err != nil {
		return false, 0, err
	}
	return linger.Onoff != 0, int(linger.Linger), socketErr
}
