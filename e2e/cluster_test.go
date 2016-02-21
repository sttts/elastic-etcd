package e2e

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/gexpect"
	"github.com/fatih/color"
	elastic "github.com/sttts/elastic-etcd/cmd/elastic-etcd"
)

type etcdProcessCluster struct {
	cfg   *elasticEtcdClusterConfig
	procs []*etcdProcess
}

type etcdProcess struct {
	cfg   *etcdProcessConfig
	proc  *gexpect.ExpectSubprocess
	donec chan struct{} // closed when Interact() terminates
}

type etcdProcessConfig struct {
	cfg         *elasticEtcdProcessConfig
	args        []string
	dataDirPath string
	clientURL   string
	isProxy     bool
}

type elasticEtcdProcessConfig struct {
	num          int
	discoveryURL string
	joinStrategy string
	name         string
	clientPort   int
	clusterSize  int
}

type elasticEtcdClusterConfig struct {
	initialClusterSize   int
	discoveryClusterSize int
	discoveryURL         string
	joinStrategy         string
}

func (c *elasticEtcdProcessConfig) etcdProcessConfig() (*etcdProcessConfig, error) {
	clientURL := fmt.Sprintf("http://localhost:%d", 2379+c.num*100)
	peerURL := fmt.Sprintf("http://localhost:%d", 2380+c.num*100)

	// run elastic-etcd
	args := []string{
		"elastic-etcd",
		"-discovery-url=" + c.discoveryURL,
		"-o=flags",
		"--initial-advertise-peer-urls", peerURL,
		fmt.Sprintf("-name=%d", c.num),
	}
	if c.joinStrategy != "" {
		args = append(args, "-join-strategy="+c.joinStrategy)
	}
	if c.clientPort != 0 {
		args = append(args, fmt.Sprintf("-client-port=%d", c.clientPort))
	}
	if c.clusterSize != 0 {
		args = append(args, fmt.Sprintf("-cluster-size=%d", c.clusterSize))
	}

	r, _, err := elastic.Run(args)
	if err != nil {
		return nil, err
	}

	// build etcd flags
	args = r.Flags()
	args = append(args, "--listen-client-urls", clientURL)
	args = append(args, "--advertise-client-urls", clientURL)
	args = append(args, "--listen-peer-urls", peerURL)

	return &etcdProcessConfig{
		cfg:         c,
		args:        args,
		dataDirPath: fmt.Sprintf("%d.etcd", c.num),
		clientURL:   clientURL,
		isProxy:     false,
	}, nil
}

func (cc *elasticEtcdClusterConfig) etcdProcessConfigs() ([]*etcdProcessConfig, error) {
	// get new discovery token
	resp, err := http.Get(fmt.Sprintf("https://discovery.etcd.io/new?size=%d", cc.discoveryClusterSize))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code %d in discovery new call: %s", resp.StatusCode, body)
	}
	discoveryURL := string(body)

	// create etcd config
	pcs := make([]*etcdProcessConfig, 0, cc.discoveryClusterSize)
	for i := 0; i < cc.initialClusterSize; i++ {
		epc := &elasticEtcdProcessConfig{
			num:          i,
			joinStrategy: cc.joinStrategy,
			clientPort:   2379,
			clusterSize:  cc.discoveryClusterSize,
			discoveryURL: discoveryURL,
		}

		pc, err := epc.etcdProcessConfig()
		if err != nil {
			return nil, err
		}

		pcs = append(pcs, pc)
	}

	return pcs, nil
}

func (etcdp *etcdProcess) logAndFind(readyStr string) error {
	i := int64(etcdp.cfg.cfg.num)
	lineColor := color.New(color.Attribute(int64(color.FgRed) + i))
	lineColor.EnableColor()
	scolorizef := lineColor.SprintfFunc()

	for {
		l, err := etcdp.proc.ReadLine()
		if err != nil {
			err = fmt.Errorf("couldn't get expected output for %d: '%s'", etcdp.cfg.cfg.num, readyStr)
			return err
		}

		log.Print(scolorizef("%d: %s", etcdp.cfg.cfg.num, l))

		if matched, err := regexp.MatchString(readyStr, l); err != nil || !matched {
			continue
		}

		go func() {
			for {
				l, err := etcdp.proc.ReadLine()
				if err != nil {
					log.Printf("instance %d terminated with %v\n", etcdp.cfg.cfg.num, err)
					close(etcdp.donec)
					break
				}
				log.Print(scolorizef("%d: %s", etcdp.cfg.cfg.num, l))
			}
		}()

		return nil
	}
}

func (etcdp *etcdProcess) waitForLaunch() error {
	readyStr := "(etcdserver: set the initial cluster version to|became follower at term)"
	err := etcdp.logAndFind(readyStr)
	if err != nil {
		return err
	}

	log.Printf("instance %d launched", etcdp.cfg.cfg.num)
	return nil
}

func newElasticEtcdProcessCluster(
	cfg *elasticEtcdClusterConfig,
) (*etcdProcessCluster, error) {
	etcdCfgs, err := cfg.etcdProcessConfigs()
	if err != nil {
		return nil, err
	}
	epc := &etcdProcessCluster{
		cfg:   cfg,
		procs: make([]*etcdProcess, cfg.discoveryClusterSize),
	}

	// launch etcd processes
	for i := range etcdCfgs {
		if err := os.RemoveAll(etcdCfgs[i].dataDirPath); err != nil {
			return nil, err
		}
		proc, err := newEtcdProcess(etcdCfgs[i])
		if err != nil {
			_ = epc.Close()
			return nil, err
		}
		epc.procs[i] = proc
	}

	// wait for cluster to start
	readyC := make(chan error, cfg.discoveryClusterSize)
	for i := range etcdCfgs {
		go func(etcdp *etcdProcess) {
			readyC <- etcdp.waitForLaunch()
		}(epc.procs[i])
	}
	for range etcdCfgs {
		if err := <-readyC; err != nil {
			_ = epc.Close()
			return nil, err
		}
	}
	return epc, nil
}

func newEtcdProcess(cfg *etcdProcessConfig) (*etcdProcess, error) {
	_, filename, _, _ := runtime.Caller(1)
	etcd := path.Join(path.Dir(filename), "../../../../../bin/etcd")

	if fileutil.Exist(etcd) == false {
		return nil, fmt.Errorf("could not find etcd binary at %s", etcd)
	}
	child, err := spawnCmd(append([]string{etcd}, cfg.args...))
	if err != nil {
		return nil, err
	}
	return &etcdProcess{cfg: cfg, proc: child, donec: make(chan struct{})}, nil
}

func (epc *etcdProcessCluster) Close() (err error) {
	log.Println("Terminating cluster")
	for _, p := range epc.procs {
		if p == nil {
			continue
		}
		_ = os.RemoveAll(p.cfg.dataDirPath)
		if curErr := p.proc.Close(); curErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; %v", err, curErr)
			} else {
				err = curErr
			}
		}
		<-p.donec
	}
	return err
}

func spawnCmd(args []string) (*gexpect.ExpectSubprocess, error) {
	// redirect stderr to stdout since gexpect only uses stdout
	cmd := `/bin/sh -c "` + strings.Join(args, " ") + ` 2>&1 "`
	return gexpect.Spawn(cmd)
}
