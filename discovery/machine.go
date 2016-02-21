package discovery

import (
	"fmt"
	"strings"

	"github.com/coreos/etcd/client"
)

// Machine represents a cluster member extracted from a discovery url.
type Machine struct {
	client.Member
}

// NewDiscoveryNode parses a discovery URL machine value into a Machine.
func NewDiscoveryNode(namedPeerURLs string, clientPort int) (*Machine, error) {
	urls := strings.Split(namedPeerURLs, ",")
	n := Machine{
		Member: client.Member{
			PeerURLs:   make([]string, 0, len(urls)),
			ClientURLs: make([]string, 0, len(urls)),
		},
	}
	for _, namedPeerURL := range urls {
		eqc := strings.SplitN(namedPeerURL, "=", 2)
		if n.Name != "" && n.Name != eqc[0] {
			return nil, fmt.Errorf("different names in %s", namedPeerURLs)
		}
		n.Name = eqc[0]
		colc := strings.SplitN(eqc[1], ":", 3)
		n.PeerURLs = append(n.PeerURLs, eqc[1])
		n.ClientURLs = append(n.ClientURLs, fmt.Sprintf("%s:%s:%d", colc[0], colc[1], clientPort))
	}

	return &n, nil
}

// NamedPeerURLs returnes a slace of name=http://domain:port values for a Machine.
func (n *Machine) NamedPeerURLs() []string {
	us := make([]string, 0, len(n.PeerURLs))
	for _, u := range n.PeerURLs {
		us = append(us, fmt.Sprintf("%s=%s", n.Name, u))
	}
	return us
}
