package traversal

import (
	"fmt"
	"io"
	"net"

	log "github.com/cihub/seelog"
)

const proxyPrefix = "[NATProxy] "

// TODO: make NATProxy universal for any transport service
type NATProxy struct {
	servicePort int
}

func NewNATProxy() *NATProxy {
	return &NATProxy{}
}

func (np *NATProxy) handOff(incomingConn *net.UDPConn) {
	proxyConn, err := np.getConnection()
	if err != nil {
		log.Error("failed to connect to NATProxy: ", err)
	}
	log.Info(proxyPrefix, "handing off connection to: ", proxyConn)
	go func() {
		defer incomingConn.Close()
		defer proxyConn.Close()
		totalBytes, err := io.Copy(proxyConn, incomingConn)
		if err != nil {
			log.Error("failed to copy stream to NATProxy: ", err)
		}
		log.Info(proxyPrefix, "total bytes incoming from client: ", totalBytes)
	}()
	go func() {
		defer incomingConn.Close()
		defer proxyConn.Close()
		totalBytes, err := io.Copy(incomingConn, proxyConn)
		if err != nil {
			log.Error("failed to copy stream to NATProxy: ", err)
		}
		log.Info(proxyPrefix, "total bytes outgoing to client: ", totalBytes)
	}()
}

func (np *NATProxy) setServicePort(port int) {
	np.servicePort = port
}

func (np *NATProxy) getConnection() (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("127.0.0.1:%d", np.servicePort))
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
