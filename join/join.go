package join

import (
	"net/http"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/client"
	"github.com/davecgh/go-spew/spew"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/rafthttp"
	"golang.org/x/net/context/ctxhttp"
	"sync"
)

const (
	LivenessTimeout = time.Second * 5
	EtcdTimeout = time.Second * 5
	DiscoveryTimeout = time.Second * 10
)

type Result struct {
	InitialCluster []string
	InitialClusterState string
	AdvertisePeerUrls string
	Discovery string
	Name string
}

func alive(ctx context.Context, url string) bool {
	ctx, _ = context.WithTimeout(ctx, LivenessTimeout)
	glog.V(6).Infof("Testing liveness of %s", url)
	resp, err := ctxhttp.Get(ctx, http.DefaultClient, url + rafthttp.ProbingPrefix)
	return err == nil && resp.StatusCode == http.StatusOK
}

func active(ctx context.Context, peerUrl string, clientPort int) (bool, error) {
	ctx, _ = context.WithTimeout(ctx, EtcdTimeout)

	uc := strings.Split(peerUrl, ":")
	url := fmt.Sprintf("%s:%s:%d", uc[0], uc[1], clientPort)

	c, err := client.New(client.Config{
		Endpoints: []string{url},
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: 5*time.Second,
	})
	if err != nil {
		return false, err
	}
	mapi := client.NewMembersAPI(c)
	glog.V(6).Infof("Testing whether %s knows the leader", url)
	leader, err := mapi.Leader(ctx)
	if err != nil {
		return false, err
	}
	return leader != nil, nil
}

type node struct{ name, url string }

func clusterExistingHeuristic(
	ctx context.Context,
	size int, nodes []node,
	clientPort int,
) (bool, []node, error) {
	quorum := size / 2 + 1

	if nodes == nil {
		glog.V(4).Infof("No nodes found in discovery service. Assuming new cluster.")
		return false, nil, nil
	}

	if len(nodes) < quorum {
		glog.V(4).Infof(
			"Only %d nodes found in discovery service, less than a quorum of %d. Assuming new cluster.",
			len(nodes),
			quorum,
		)
		return false, nil, nil
	}

	if len(nodes) == size {
		glog.V(4).Infof("Cluster is full. Assuming existing cluster.")
		return true, nil, nil
	}

	wg := sync.WaitGroup{}
	wg.Add(len(nodes))
	activeNodes := make([]node, 0, len(nodes))
	for _, n := range nodes {
		go func(n node) {
			defer wg.Done()
			if !alive(ctx, n.url) {
				glog.Infof("Node %s looks dead", n.url)
				return
			}
			if ok, err := active(ctx, n.url, clientPort); !ok {
				if err != nil {
					glog.Error(err)
				}
				glog.Infof("Node %s is not in a healthy cluster.", n.url)
				return
			}
			glog.Infof("Node %s looks alive and active in a cluster", n.url)
			activeNodes = append(activeNodes, n)
		}(n)
	}
	wg.Wait()

	if len(activeNodes) > 0 {
		return true, activeNodes, nil
	}

	return false, nil, nil
}

func discoveryValue(ctx context.Context, baseUrl, key string) (*store.Event, error) {
	ctx, _ = context.WithTimeout(ctx, DiscoveryTimeout)

	url := baseUrl + key
	glog.V(6).Infof("Getting %s", url)
	resp, err := ctxhttp.Get(ctx, http.DefaultClient, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("status code %d from %q: %s", resp.StatusCode, url, body)
	}

	var res store.Event
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("invalid answer from %q: %v", url, err)
	}

	glog.V(9).Infof("Got: %s", spew.Sdump(res))

	return &res, nil
}

func deleteDiscoveryMachine(ctx context.Context, baseUrl, id string) error {
	ctx, _ = context.WithTimeout(ctx, DiscoveryTimeout)

	url := baseUrl + "/" + strings.TrimLeft(id, "/")
	req, err := http.NewRequest("DELETE", url, strings.NewReader(""))
	if err != nil {
		return err
	}
	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("status code %d on DELETE for %q: %s", resp.StatusCode, url, body)
	}

	return nil
}

func Join(url, name, initialAdvertisePeerUrls string, clientPort int) (*Result, error) {
	ctx := context.Background()

	res, err := discoveryValue(ctx, url, "/")
	if err != nil {
		return nil, err
	}
	nodes := make([]node, 0, len(res.Node.Nodes))
	for _, nn := range res.Node.Nodes {
		nnc := strings.Split(*nn.Value, "=")
		nodes = append(nodes, node{nnc[0], nnc[1]})
	}

	res, err = discoveryValue(ctx, url, "/_config/size")
	if err != nil {
		return nil, err
	}

	size, err := strconv.ParseInt(*res.Node.Value, 10, 64)
	glog.V(2).Infof("Got a target cluster size of %d from the discovery url", size)

	existing, activeNodes, err := clusterExistingHeuristic(ctx, int(size), nodes, clientPort)
	if err != nil {
		return nil, err
	}
	if existing {
		activeNodeNames := make([]string, 0, len(nodes))
		for _, n := range activeNodes {
			activeNodeNames = append(activeNodeNames, fmt.Sprintf("%s=%s", n.name, n.url))
		}

		advertisedNamedUrls := make([]string, 0, len(initialAdvertisePeerUrls))
		for _, u := range strings.Split(initialAdvertisePeerUrls, ",") {
			advertisedNamedUrls = append(advertisedNamedUrls, fmt.Sprintf("%s=%s", name, u))
		}

		return &Result{
			InitialCluster: append(advertisedNamedUrls, activeNodeNames...),
			InitialClusterState: "existing",
			AdvertisePeerUrls: initialAdvertisePeerUrls,
			Name: name,
		}, nil
	} else {
		return &Result{
			InitialClusterState: "new",
			Discovery: url,
			Name: name,
		}, nil
	}
}
