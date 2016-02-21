package e2e

import (
	"os"
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/pborman/uuid"
	"github.com/sttts/elastic-etcd/join"
	"golang.org/x/net/context"
)

func smokeTest(t *testing.T, epc *etcdProcessCluster) {
	cfg := client.Config{
		Endpoints: []string{},
	}
	for _, p := range epc.procs {
		cfg.Endpoints = append(cfg.Endpoints, p.cfg.clientURL)
	}
	c, err := client.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	kapi := client.NewKeysAPI(c)

	v := uuid.New()
	_, err = kapi.Set(context.Background(), "/foo", v, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := kapi.Get(context.Background(), "/foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Node == nil || resp.Node.Value != v {
		t.Fatalf("Invalid value %q for previously written key /foo", v)
	}
}

func launchCluster(t *testing.T, cfg *elasticEtcdClusterConfig) *etcdProcessCluster {
	epc, err := newElasticEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}

	smokeTest(t, epc)

	return epc
}

func testNewCluster(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer func() { _ = epc.Close() }()
}

func testRestartEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer func() { _ = epc.Close() }()

	// stop process
	_ = epc.procs[2].proc.Close()

	// restart etcd with same flags
	proc, err := newEtcdProcess(epc.procs[2].cfg)
	if err != nil {
		t.Fatal(err)
	}
	epc.procs[2] = proc

	// wait for join
	err = epc.procs[2].waitForLaunch()
	if err != nil {
		t.Fatal(err)
	}

	smokeTest(t, epc)
}

func testRestartElasticEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer func() { _ = epc.Close() }()

	// stop process
	_ = epc.procs[2].proc.Close()

	// restart elastic-etcd and then etcd with the new flags
	newCfg, err := epc.procs[2].cfg.cfg.etcdProcessConfig()
	if err != nil {
		t.Fatal(err)
	}
	epc.procs[2], err = newEtcdProcess(newCfg)
	if err != nil {
		t.Fatal(err)
	}

	// wait for join
	err = epc.procs[2].waitForLaunch()
	if err != nil {
		t.Fatal(err)
	}

	smokeTest(t, epc)
}

func testRestartCleanElasticEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer func() { _ = epc.Close() }()

	// stop process
	_ = epc.procs[2].proc.Close()

	// clean data dir
	if err := os.RemoveAll(epc.procs[2].cfg.dataDirPath); err != nil {
		t.Fatal(err)
	}

	// restart elastic-etcd and then etcd with the new flags
	newCfg, err := epc.procs[2].cfg.cfg.etcdProcessConfig()
	if err != nil {
		t.Fatal(err)
	}
	epc.procs[2], err = newEtcdProcess(newCfg)
	if err != nil {
		t.Fatal(err)
	}

	// wait for join
	err = epc.procs[2].waitForLaunch()
	if err != nil {
		t.Fatal(err)
	}

	smokeTest(t, epc)
}

func testGrowElasticEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer func() { _ = epc.Close() }()

	// start a 4th etcd, increase cluster size
	eepc := *epc.procs[2].cfg.cfg
	eepc.num = 3
	eepc.clusterSize = 5

	newCfg, err := eepc.etcdProcessConfig()
	if err != nil {
		t.Fatal(err)
	}
	ep, err := newEtcdProcess(newCfg)
	if err != nil {
		t.Fatal(err)
	}
	epc.procs = append(epc.procs, ep)

	// wait for join
	err = ep.waitForLaunch()
	if err != nil {
		t.Fatal(err)
	}

	smokeTest(t, epc)
}

var (
	elasticReplaceConfig = elasticEtcdClusterConfig{
		initialClusterSize:   3,
		discoveryClusterSize: 3,
		joinStrategy:         string(join.ReplaceStrategy),
	}

	elasticPruneConfig = elasticEtcdClusterConfig{
		initialClusterSize:   3,
		discoveryClusterSize: 3,
		joinStrategy:         string(join.PruneStrategy),
	}

	elasticAddConfig = elasticEtcdClusterConfig{
		initialClusterSize:   3,
		discoveryClusterSize: 3,
		joinStrategy:         string(join.AddStrategy),
	}

	elasticPreparedConfig = elasticEtcdClusterConfig{
		initialClusterSize:   3,
		discoveryClusterSize: 3,
		joinStrategy:         string(join.PreparedStrategy),
	}
)

func TestElasticNewClusterPrepared(t *testing.T) { testNewCluster(t, &elasticPreparedConfig) }
func TestElasticNewClusterReplace(t *testing.T)  { testNewCluster(t, &elasticReplaceConfig) }
func TestElasticNewClusterPrune(t *testing.T)    { testNewCluster(t, &elasticPruneConfig) }
func TestElasticNewClusterAdd(t *testing.T)      { testNewCluster(t, &elasticAddConfig) }

func TestElasticRestartEtcdPrepared(t *testing.T) { testRestartEtcd(t, &elasticPreparedConfig) }
func TestElasticRestartEtcdReplace(t *testing.T)  { testRestartEtcd(t, &elasticReplaceConfig) }
func TestElasticRestartEtcdPrune(t *testing.T)    { testRestartEtcd(t, &elasticPruneConfig) }
func TestElasticRestartEtcdAdd(t *testing.T)      { testRestartEtcd(t, &elasticAddConfig) }

func TestElasticRestartElasticEtcdPrepared(t *testing.T) {
	testRestartElasticEtcd(t, &elasticPreparedConfig)
}
func TestElasticRestartElasticEtcdReplace(t *testing.T) {
	testRestartElasticEtcd(t, &elasticReplaceConfig)
}
func TestElasticRestartElasticEtcdPrune(t *testing.T) {
	testRestartElasticEtcd(t, &elasticPruneConfig)
}
func TestElasticRestartElasticEtcdAdd(t *testing.T) {
	testRestartElasticEtcd(t, &elasticAddConfig)
}

func TestElasticRestartCleanElasticEtcdReplace(t *testing.T) {
	testRestartCleanElasticEtcd(t, &elasticReplaceConfig)
}
func TestElasticRestartCleanElasticEtcdPrune(t *testing.T) {
	testRestartCleanElasticEtcd(t, &elasticPruneConfig)
}

func TestElasticGrowElasticEtcdReplace(t *testing.T) {
	testGrowElasticEtcd(t, &elasticReplaceConfig)
}
func TestElasticGrowElasticEtcdPrune(t *testing.T) {
	testGrowElasticEtcd(t, &elasticPruneConfig)
}
func TestElasticGrowElasticEtcdAdd(t *testing.T) {
	testGrowElasticEtcd(t, &elasticAddConfig)
}
