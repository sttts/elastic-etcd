package e2e

import (
	"testing"

	"github.com/coreos/etcd/pkg/testutil"
	elastic "github.com/sttts/elastic-etcd"
	"fmt"
)

type elasticEtcdProcessConfig struct {
	discoveryUrl             string
	joinStrategy             string
	name                     string
	clientPort               int
	clusterSize              int
	initialAdvertisePeerUrls string
}

func (c* elasticEtcdProcessConfig) etcdProcessConfig() *etcdProcessConfig {
	args := []string{"elastic-etcd", "-discovery-url=c.discoveryUrl", "--v=6", "--logtostderr", "join", "-o=flags"}
	if c.joinStrategy {
		args = append(args, "-join-strategy=" + c.joinStrategy)
	}
	if c.name {
		args = append(args, "-name=" + c.name)
	}
	if c.clientPort != 0 {
		args = append(args, fmt.Sprintf("-client-port=%d", c.clientPort))
	}
	if c.clusterSize != 0 {
		args = append(args, fmt.Sprintf("-cluster-size=%d", c.clusterSize))
	}
	elastic.Run(args)
}

func test_newCluster(t *testing.T, cfg *etcdProcessClusterConfig) {
	defer testutil.AfterTest(t)

	epc, err := newEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("error closing etcd processes (%v)", err)
		}
	}()

	expectPut := `{"action":"set","node":{"key":"/testKey","value":"foo","`
	if err := cURLPut(epc, "testKey", "foo", expectPut); err != nil {
		t.Fatalf("failed put with curl (%v)", err)
	}

	expectGet := `{"action":"get","node":{"key":"/testKey","value":"foo","`
	if err := cURLGet(epc, "testKey", expectGet); err != nil {
		t.Fatalf("failed get with curl (%v)", err)
	}
}

func newElasticEtcdProcessCluster(cfg *elasticEtcdClusterConfig) (*etcdProcessCluster, error) {
	etcdCfgs := cfg.etcdProcessConfigs()
	epc := &etcdProcessCluster{
		cfg:   cfg,
		procs: make([]*etcdProcess, cfg.clusterSize+cfg.proxySize),
	}

	// launch etcd processes
	for i := range etcdCfgs {
		proc, err := newEtcdProcess(etcdCfgs[i])
		if err != nil {
			epc.Close()
			return nil, err
		}
		epc.procs[i] = proc
	}

	// wait for cluster to start
	readyC := make(chan error, cfg.clusterSize+cfg.proxySize)
	readyStr := "etcdserver: set the initial cluster version to"
	for i := range etcdCfgs {
		go func(etcdp *etcdProcess) {
			rs := readyStr
			if etcdp.cfg.isProxy {
				// rs = "proxy: listening for client requests on"
				rs = "proxy: endpoints found"
			}
			ok, err := etcdp.proc.ExpectRegex(rs)
			if err != nil {
				readyC <- err
			} else if !ok {
				readyC <- fmt.Errorf("couldn't get expected output: '%s'", rs)
			} else {
				readyC <- nil
			}
			etcdp.proc.ReadLine()
			etcdp.proc.Interact() // this blocks(leaks) if another goroutine is reading
			etcdp.proc.ReadLine() // wait for leaky goroutine to accept an EOF
			close(etcdp.donec)
		}(epc.procs[i])
	}
	for range etcdCfgs {
		if err := <-readyC; err != nil {
			epc.Close()
			return nil, err
		}
	}
	return epc, nil
}