package e2e

import (
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/pborman/uuid"
	"github.com/sttts/elastic-etcd/join"
	"golang.org/x/net/context"
	"os"
)

func smokeTest(t *testing.T, epc *etcdProcessCluster) {
	cfg := client.Config{
		Endpoints: []string{},
	}
	for _, p := range epc.procs {
		cfg.Endpoints = append(cfg.Endpoints, p.cfg.clientUrl)
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

func test_newCluster(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer epc.Close()
}

func test_restartEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer epc.Close()

	// stop process
	epc.procs[2].proc.Close()

	// restart etcd with same flags
	proc, err := newEtcdProcess(epc.procs[2].cfg)
	if err != nil {
		epc.Close()
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

func test_restartElasticEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer epc.Close()

	// stop process
	epc.procs[2].proc.Close()

	// restart elastic-etcd and then etcd with the new flags
	newCfg, err := epc.procs[2].cfg.cfg.etcdProcessConfig()
	if err != nil {
		t.Fatal(err)
	}
	epc.procs[2], err = newEtcdProcess(newCfg)
	if err != nil {
		epc.Close()
		t.Fatal(err)
	}

	// wait for join
	err = epc.procs[2].waitForLaunch()
	if err != nil {
		t.Fatal(err)
	}

	smokeTest(t, epc)
}

func test_restartCleanElasticEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer epc.Close()

	// stop process
	epc.procs[2].proc.Close()

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
		epc.Close()
		t.Fatal(err)
	}

	// wait for join
	err = epc.procs[2].waitForLaunch()
	if err != nil {
		t.Fatal(err)
	}

	smokeTest(t, epc)
}

func test_growElasticEtcd(t *testing.T, cfg *elasticEtcdClusterConfig) {
	epc := launchCluster(t, cfg)
	defer epc.Close()

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
	if err != nil {
		epc.Close()
		t.Fatal(err)
	}

	// wait for join
	ep.waitForLaunch()
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

func Test_elastic_newCluster_Prepared(t *testing.T) { test_newCluster(t, &elasticPreparedConfig) }
func Test_elastic_newCluster_Replace(t *testing.T)  { test_newCluster(t, &elasticReplaceConfig) }
func Test_elastic_newCluster_Prune(t *testing.T)    { test_newCluster(t, &elasticPruneConfig) }
func Test_elastic_newCluster_Add(t *testing.T)      { test_newCluster(t, &elasticAddConfig) }

func Test_elastic_restartEtcd_Prepared(t *testing.T) { test_restartEtcd(t, &elasticPreparedConfig) }
func Test_elastic_restartEtcd_Replace(t *testing.T)  { test_restartEtcd(t, &elasticReplaceConfig) }
func Test_elastic_restartEtcd_Prune(t *testing.T)    { test_restartEtcd(t, &elasticPruneConfig) }
func Test_elastic_restartEtcd_Add(t *testing.T)      { test_restartEtcd(t, &elasticAddConfig) }

func Test_elastic_restartElasticEtcd_Prepared(t *testing.T) {
	test_restartElasticEtcd(t, &elasticPreparedConfig)
}
func Test_elastic_restartElasticEtcd_Replace(t *testing.T) {
	test_restartElasticEtcd(t, &elasticReplaceConfig)
}
func Test_elastic_restartElasticEtcd_Prune(t *testing.T) {
	test_restartElasticEtcd(t, &elasticPruneConfig)
}
func Test_elastic_restartElasticEtcd_Add(t *testing.T) { test_restartElasticEtcd(t, &elasticAddConfig) }

func Test_elastic_restartCleanElasticEtcd_Replace(t *testing.T) {
	test_restartCleanElasticEtcd(t, &elasticReplaceConfig)
}
func Test_elastic_restartCleanElasticEtcd_Prune(t *testing.T) {
	test_restartCleanElasticEtcd(t, &elasticPruneConfig)
}

func Test_elastic_growElasticEtcd_Replace(t *testing.T) {
	test_growElasticEtcd(t, &elasticReplaceConfig)
}
func Test_elastic_growElasticEtcd_Prune(t *testing.T) { test_growElasticEtcd(t, &elasticPruneConfig) }
func Test_elastic_growElasticEtcd_Add(t *testing.T)   { test_growElasticEtcd(t, &elasticAddConfig) }
