package api

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
	"syscall"
)

// SocketListener listens on a unix socket at path that only the given user ids
// may use: the kernel reports who opened each connection (SO_PEERCRED), and
// any other user's connection is closed unread. The socket file is mode 0660,
// in group gid when gid >= 0, so other users can't even connect.
func SocketListener(path string, gid int, allow []int) (net.Listener, error) {
	if len(allow) == 0 {
		return nil, errors.New("the socket needs at least one allowed user id")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if gid >= 0 {
		if err := os.Chown(path, -1, gid); err != nil {
			l.Close()
			return nil, fmt.Errorf("socket group: %w", err)
		}
	}
	if err := os.Chmod(path, 0o660); err != nil {
		l.Close()
		return nil, err
	}
	return &peerListener{Listener: l, allow: allow}, nil
}

type peerListener struct {
	net.Listener
	allow []int
}

func (p *peerListener) Accept() (net.Conn, error) {
	for {
		c, err := p.Listener.Accept()
		if err != nil {
			return nil, err
		}
		uid, err := peerUID(c)
		if err == nil && slices.Contains(p.allow, uid) {
			return c, nil
		}
		log.Printf("socket: refused a connection from uid %d (%v)", uid, err)
		c.Close()
	}
}

func peerUID(c net.Conn) (int, error) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return -1, errors.New("not a unix connection")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return -1, err
	}
	var cred *syscall.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return -1, err
	}
	if credErr != nil {
		return -1, credErr
	}
	return int(cred.Uid), nil
}
