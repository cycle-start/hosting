package cli

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// ProxyTarget describes a service to proxy from localhost to the tunnel.
type ProxyTarget struct {
	Service   ServiceEntry
	LocalPort int
}

// StartProxy listens on localhost:localPort and forwards connections through the tunnel
// to the remote service address.
func StartProxy(tunnel *Tunnel, target ProxyTarget) (net.Listener, error) {
	remoteAddr := fmt.Sprintf("[%s]:%d", target.Service.Address, target.Service.RemotePort())
	localAddr := fmt.Sprintf("127.0.0.1:%d", target.LocalPort)

	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", localAddr, err)
	}

	go func() {
		for {
			local, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			go handleProxy(tunnel, local, remoteAddr)
		}
	}()

	return listener, nil
}

func handleProxy(tunnel *Tunnel, local net.Conn, remoteAddr string) {
	defer local.Close()

	remote, err := tunnel.DialTCP(remoteAddr)
	if err != nil {
		log.Printf("tunnel dial %s: %v", remoteAddr, err)
		return
	}
	defer remote.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(remote, local)
	}()

	go func() {
		defer wg.Done()
		io.Copy(local, remote)
	}()

	wg.Wait()
}
