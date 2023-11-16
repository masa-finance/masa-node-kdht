package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

type NodeLite struct {
	Host       host.Host
	PrivKey    crypto.PrivKey
	Protocol   protocol.ID
	multiAddrs multiaddr.Multiaddr
	DHT        *dht.IpfsDHT
	Context    context.Context
	PeerChan   chan PeerEvent
}

func NewNodeLite(privKey crypto.PrivKey, ctx context.Context) (*NodeLite, error) {
	// Start with the default scaling limits.
	scalingLimits := rcmgr.DefaultLimits
	concreteLimits := scalingLimits.AutoScale()
	limiter := rcmgr.NewFixedLimiter(concreteLimits)

	resourceManager, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		return nil, err
	}

	addrStr := []string{
		"/ip4/0.0.0.0/tcp/0",
	}
	if os.Getenv(PortNbr) != "" {
		addrStr = []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", os.Getenv(PortNbr)),
		}
	}

	host, err := libp2p.New(
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		libp2p.ListenAddrStrings(addrStr...),
		libp2p.Identity(privKey),
		libp2p.ResourceManager(resourceManager),
		libp2p.Ping(false), // disable built-in ping
		libp2p.ChainOptions(
			libp2p.Security(noise.ID, noise.New),
			libp2p.Security(libp2ptls.ID, libp2ptls.New),
		),
	)
	if err != nil {
		return nil, err
	}
	ma, err := GetMultiAddressForHost(host)
	if err != nil {
		return nil, err
	}
	return &NodeLite{
		Host:       host,
		PrivKey:    privKey,
		multiAddrs: ma,
		Context:    ctx,
		PeerChan:   make(chan PeerEvent),
	}, nil
	return nil, nil
}

func (node *NodeLite) Start() (err error) {
	logrus.Infof("Starting node with ID: %s", node.multiAddrs.String())
	node.Host.SetStreamHandler(node.Protocol, node.handleStream)
	go node.handleDiscoveredPeers()

	//myNetwork.WithMDNS(node.Host, rendezvous, node.PeerChan)
	bootNodeAddrs, err := GetBootNodesMultiAddress(os.Getenv(Peers))
	if err != nil {
		return err
	}
	node.DHT, err = WithDht(node.Context, node.Host, bootNodeAddrs, node.Protocol, node.PeerChan)
	if err != nil {
		return err
	}
	return nil
}

func (node *NodeLite) handleDiscoveredPeers() {
	for {
		select {
		case peer := <-node.PeerChan: // will block until we discover a peer
			logrus.Info("Peer Event for:", peer, ", Action:", peer.Action)

			if err := node.Host.Connect(node.Context, peer.AddrInfo); err != nil {
				logrus.Error("Connection failed:", err)
				continue
			}

			// open a stream, this stream will be handled by handleStream other end
			stream, err := node.Host.NewStream(node.Context, peer.AddrInfo.ID, node.Protocol)

			if err != nil {
				logrus.Error("Stream open failed", err)
			} else {
				rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

				go node.writeData(rw, peer)
				go node.readData(rw, peer)
			}
		case <-node.Context.Done():
			return
		}
	}
}

func (node *NodeLite) handleStream(stream network.Stream) {
	logrus.Info("Got a new stream!")

	remotePeer := stream.Conn().RemotePeer()

	// Check if we're already connected to the peer
	connStatus := node.Host.Network().Connectedness(remotePeer)
	if connStatus != network.Connected {
		// We're not connected to the peer, so try to establish a connection
		ctx, cancel := context.WithTimeout(node.Context, 5*time.Second)
		defer cancel()
		err := node.Host.Connect(ctx, peer.AddrInfo{ID: remotePeer})
		if err != nil {
			logrus.Warningf("Failed to connect to peer %s: %v", remotePeer, err)
			return
		}
	}

	// Try to add the peer to the routing table (no-op if already present).
	added, err := node.DHT.RoutingTable().TryAddPeer(remotePeer, true, true)
	if err != nil {
		logrus.Warningf("Failed to add peer %s to routing table: %v", remotePeer, err)
	} else if !added {
		logrus.Warningf("Failed to add peer %s to routing table", remotePeer)
	} else {
		logrus.Infof("Successfully added peer %s to routing table", remotePeer)
	}
	// Check if the peer is useful after trying to add it
	isUsefulAfter := node.DHT.RoutingTable().UsefulNewPeer(remotePeer)
	logrus.Infof("Is peer %s useful after adding: %v", remotePeer, isUsefulAfter)

	logrus.Infof("Routing table size: %d", node.DHT.RoutingTable().Size())

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go node.readData(rw, PeerEvent{Source: "StreamHandler"})
	go node.writeData(rw, PeerEvent{Source: "StreamHandler"})

	// 'stream' will stay open until you close it (or the other side closes it).
}

func (node *NodeLite) readData(rw *bufio.ReadWriter, event PeerEvent) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			logrus.Error("Error reading from buffer:", err)
			return
		}
		if str == "" {
			return
		}
		if str != "\n" {
			msg := fmt.Sprintf("%s: readData: %s", event.Source, str)
			fmt.Printf("\x1b[32m%s\x1b[0m> ", msg)
		}
	}
}

func (node *NodeLite) writeData(rw *bufio.ReadWriter, event PeerEvent) {
	for {
		// Generate a message including the multiaddress of the sender
		sendData := fmt.Sprintf("%s: writeData: Hello from %s\n", event.Source, node.multiAddrs.String())

		_, err := rw.WriteString(sendData)
		if err != nil {
			logrus.Error("Error writing to buffer:", err)
			return
		}
		err = rw.Flush()
		if err != nil {
			logrus.Error("Error flushing buffer:", err)
			return
		}
		// Sleep for a while before sending the next message
		time.Sleep(time.Second * 10)
	}
}
