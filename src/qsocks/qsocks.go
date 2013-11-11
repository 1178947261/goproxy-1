package qsocks

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"logging"
	"net"
)

const VERSION = 0x11

const (
	REQ_CONN = iota
	REQ_DNS
)

var logger logging.Logger

func init() {
	var err error
	logger, err = logging.NewFileLogger("default", -1, "qsocks")
	if err != nil {
		panic(err)
	}
}

func fillString(b []byte, s string) (r []byte) {
	b[0] = byte(len(s))
	copy(b[1:], []byte(s))
	return b[len(s)+1:]
}

func getString(r io.Reader) (s string, err error) {
	var size [1]byte
	_, err = r.Read(size[:])
	if err != nil {
		return
	}
	buf := make([]byte, uint8(size[0]))
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return
	}
	return string(buf), nil
}

func Auth(username, password string) (buf []byte, err error) {
	size := uint16(16 + 1 + 2 + len(username) + len(password))
	buf = make([]byte, size)

	_, err = rand.Read(buf[:16])
	if err != nil {
		return
	}
	cur := buf[16:]

	cur[0] = VERSION
	cur = cur[1:]

	cur = fillString(cur, username)
	cur = fillString(cur, password)
	return
}

func Conn(hostname string, port uint16) (buf []byte, err error) {
	size := uint16(2 + len(hostname) + 2)
	buf = make([]byte, size)
	buf[0] = REQ_CONN
	cur := buf[1:]

	cur = fillString(cur, hostname)
	binary.BigEndian.PutUint16(cur, port)
	return
}

func GetAuth(conn net.Conn) (username, password string, err error) {
	var buf [17]byte
	_, err = io.ReadFull(conn, buf[:])
	if err != nil {
		return
	}

	ver := uint8(buf[16])
	if ver < VERSION {
		err = fmt.Errorf("lower version: %d", ver)
		return
	}

	username, err = getString(conn)
	if err != nil {
		return
	}
	password, err = getString(conn)
	if err != nil {
		return
	}
	return
}

func GetReq(conn net.Conn) (req uint8, err error) {
	var buf [1]byte
	_, err = conn.Read(buf[:])
	if err != nil {
		return
	}
	return uint8(buf[0]), nil
}

func GetConn(conn net.Conn) (hostname string, port uint16, err error) {
	hostname, err = getString(conn)
	if err != nil {
		return
	}
	var buf [2]byte
	_, err = conn.Read(buf[:])
	if err != nil {
		return
	}
	port = binary.BigEndian.Uint16(buf[:])
	return
}

func SendResponse(conn net.Conn, res uint8) (err error) {
	var buf [1]byte
	buf[0] = byte(res)
	_, err = conn.Write(buf[:])
	if err != nil {
		return
	}
	return
}

func RecvResponse(conn net.Conn) (res uint8, err error) {
	var buf [1]byte
	_, err = conn.Read(buf[:])
	if err != nil {
		return
	}
	res = uint8(buf[0])
	return
}
