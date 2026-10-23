//go:build !windows

package api

import (
	"fmt"
	"net"
)

func socketLinger(net.Conn) (bool, int, error) {
	return false, 0, fmt.Errorf("not inspected on this platform")
}
