package node

import (
	"fmt"
	"strings"

	"github.com/coreos/etcd/client"
)

type DiscoveryNode struct {
	client.Member
}

func NewDiscoveryNode(namedPeerUrls string, clientPort int) (*DiscoveryNode, error) {
	urls := strings.Split(namedPeerUrls, ",")
	n := DiscoveryNode{
		Member: client.Member{
			PeerURLs:   make([]string, 0, len(urls)),
			ClientURLs: make([]string, 0, len(urls)),
		},
	}
	for _, namedPeerUrl := range urls {
		eqc := strings.SplitN(namedPeerUrl, "=", 2)
		if n.Name != "" && n.Name != eqc[0] {
			return nil, fmt.Errorf("different names in %s", namedPeerUrls)
		}
		n.Name = eqc[0]
		colc := strings.SplitN(eqc[1], ":", 3)
		n.PeerURLs = append(n.PeerURLs, eqc[1])
		n.ClientURLs = append(n.ClientURLs, fmt.Sprintf("%s:%s:%d", colc[0], colc[1], clientPort))
	}

	return &n, nil
}

func (n *DiscoveryNode) NamedPeerUrls() []string {
	us := make([]string, 0, len(n.PeerURLs))
	for _, u := range n.PeerURLs {
		us = append(us, fmt.Sprintf("%s=%s", n.Name, u))
	}
	return us
}
