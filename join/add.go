package join

import (
	"errors"
	"fmt"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"github.com/golang/glog"
	"github.com/sttts/elastic-etcd/node"
)

type MemberAdder struct {
	mapi        client.MembersAPI
	activeNodes []node.DiscoveryNode
	strategy    Strategy
	clientPort  int
	targetSize  int
}

func NewMemberAdder(
	activeNodes []node.DiscoveryNode,
	strategy Strategy,
	clientPort int,
	targetSize int,
) (*MemberAdder, error) {
	activeUrls := make([]string, 0, len(activeNodes))
	for _, an := range activeNodes {
		activeUrls = append(activeUrls, an.ClientURLs...)
	}

	c, err := client.New(client.Config{
		Endpoints:               activeUrls,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: EtcdTimeout,
	})
	if err != nil {
		return nil, err
	}

	return &MemberAdder{
		mapi:        client.NewMembersAPI(c),
		activeNodes: activeNodes,
		strategy:    strategy,
		clientPort:  clientPort,
		targetSize:  targetSize,
	}, nil
}

func (ma *MemberAdder) findUnstartedMember(
	members []client.Member,
	urls []string,
) *client.Member {
	newUrls := map[string]struct{}{}
	for _, u := range urls {
		newUrls[u] = struct{}{}
	}

findUnstartedMember:
	for _, m := range members {
		if m.Name != "" {
			continue
		}

		// check whether m has a subset of our peer urls
		for _, u := range m.PeerURLs {
			if _, found := newUrls[u]; !found {
				continue findUnstartedMember
			}
		}
		glog.Infof("Unstarted member %s with matching %v peer urls found", m.ID, m.PeerURLs)
		return &m
	}

	return nil
}

func (ma *MemberAdder) removeDeadMembersN(
	ctx context.Context,
	members []client.Member,
	maxNum int,
) ([]*client.Member, error) {
	deleted := []*client.Member{}
searchForDead:
	for _, m := range members {
		if len(deleted) >= maxNum {
			break
		}

		for _, u := range m.PeerURLs {
			n, err := node.NewDiscoveryNode(fmt.Sprintf("%s=%s", m.Name, u), ma.clientPort)
			if err != nil {
				glog.Warningf("Invalid peer URL %s in member %s found", u, m.Name)
				continue searchForDead
			}
			if alive(ctx, n.Member) {
				isActive, err := active(ctx, n.Member)
				if err != nil {
					glog.Warningf("Error checking member %s health", m.Name)
					continue searchForDead
				}
				if isActive {
					glog.V(5).Infof("Member %v found to be alive and active", n.NamedPeerUrls())
					continue searchForDead
				}
			}
		}

		glog.Infof("Trying to remove dead member %s=%q", m.Name, m.PeerURLs)
		err := ma.mapi.Remove(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		glog.Infof("Removed dead member %s=%q", m.Name, m.PeerURLs)

		break
	}

	return deleted, nil
}

func (ma *MemberAdder) protectCluster(ctx context.Context) error {
	// check that we don't destroy the quorum
	ms, err := ma.mapi.List(ctx)
	if err != nil {
		return err
	}
	startedMembers := 0
	healthyMembers := 0
	for _, m := range ms {
		if m.Name != "" {
			startedMembers++
		}
		if alive(ctx, m) {
			if isActive, err := active(ctx, m); isActive && err == nil {
				healthyMembers++
			}
		}
	}

	if startedMembers >= ma.targetSize {
		return fmt.Errorf("cluster is already full with %d members", ma.targetSize)
	}

	futureQuorum := (startedMembers+1)/2 + 1
	if healthyMembers < futureQuorum {
		return fmt.Errorf("cannot add another member temporarily to the %d member "+
			"cluster (with %d members up) because we put the future quorum %d at risk",
			startedMembers, healthyMembers, futureQuorum)
	}
	glog.Infof("Even when this new member does not successfully start up and join the cluster, "+
		"the future quorum %d is not at risk. Continuing.", futureQuorum)
	return nil
}

func (ma *MemberAdder) Add(
	ctx context.Context,
	name string,
	urls []string,
) ([]string, error) {
	ctx, _ = context.WithTimeout(ctx, EtcdTimeout)

	glog.V(4).Info("Getting cluster members")
	ms, err := ma.mapi.List(ctx)
	if err != nil {
		return nil, err
	}

	unstarted := ma.findUnstartedMember(ms, urls)
	if unstarted != nil {
		glog.Infof("Found matching member entry %s=%v, no need to add", unstarted.Name, unstarted.PeerURLs)

		if err := ma.protectCluster(ctx); err != nil {
			return nil, err
		}

		return unstarted.PeerURLs, nil
	}

	switch ma.strategy {
	case ReplaceStrategy:
		if len(ms) >= ma.targetSize {
			removed, err := ma.removeDeadMembersN(ctx, ms, 1)
			if err != nil {
				return nil, err
			}
			if len(removed) == 0 {
				return nil, errors.New("full cluster and no dead member")
			}
		} else {
			glog.Infof("Cluster not full with %d member our of %d. Going ahead with adding.", len(ms), ma.targetSize)
		}
	case PruneStrategy:
		_, err := ma.removeDeadMembersN(ctx, ms, len(ms))
		if err != nil {
			return nil, err
		}
	}

	if err := ma.protectCluster(ctx); err != nil {
		return nil, err
	}

	// add first of our peer urls. We cannot add all because we have to decide later which
	// one is stated in the initial-cluster parameter. That one will be used to compute the
	// member id.
	glog.V(4).Infof("Trying to add member with peer url %s", urls[0])
	_, err = ma.mapi.Add(ctx, urls[0])
	if err != nil {
		return nil, err
	}
	glog.Infof("Added member with peer url %s", urls[0])

	return []string{urls[0]}, nil
}
