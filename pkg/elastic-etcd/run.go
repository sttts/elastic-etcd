package elastic

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/golang/glog"
	"github.com/sttts/elastic-etcd/cliext"
	"github.com/sttts/elastic-etcd/join"
)

// EtcdConfig is the result of the elastic-etcd algorithm, turned into etcd flags or env vars.
type EtcdConfig struct {
	join.EtcdConfig
	DataDir string
}

// Flags turns an EtcdConfig struct into etcd flags.
func (r *EtcdConfig) Flags() []string {
	args := []string{}
	if r.InitialClusterState != "" {
		args = append(args, fmt.Sprintf("-initial-cluster-state=%s", r.InitialClusterState))
	}
	if r.InitialCluster != nil {
		args = append(args, fmt.Sprintf("-initial-cluster=%s", strings.Join(r.InitialCluster, ",")))
	}
	if r.Discovery != "" {
		args = append(args, fmt.Sprintf("-discovery=%s", r.Discovery))
	}
	if r.AdvertisePeerURLs != "" {
		args = append(args, fmt.Sprintf("-initial-advertise-peer-urls=%s", r.AdvertisePeerURLs))
	}

	args = append(args, fmt.Sprintf("-name=%s", r.Name))
	args = append(args, fmt.Sprintf("-data-dir=%s", r.DataDir))

	glog.V(4).Infof("Derived etcd parameter: %v", args)
	return args
}

// Run starts the elastic-etcd algorithm on the given flags and return an EtcdConfig and the
// output format.
func Run(args []string) (*EtcdConfig, string, error) {
	var (
		discoveryURL             string
		joinStrategy             string
		format                   string
		name                     string
		clientPort               int
		clusterSize              int
		initialAdvertisePeerURLs string
		dataDir                  string
	)

	var formats = []string{"env", "dropin", "flags"}
	var strategies = []string{
		string(join.PreparedStrategy),
		string(join.ReplaceStrategy),
		string(join.PruneStrategy),
		string(join.AddStrategy),
	}

	checkFlags := func() error {
		if name == "" {
			return errors.New("name must be set")
		}
		if initialAdvertisePeerURLs == "" {
			return errors.New("initial-advertise-peer-urls must consist at least of one url")
		}
		if discoveryURL == "" {
			return errors.New("discovery-url must be set")
		}

		discoveryURL = strings.TrimRight(discoveryURL, "/")

		u, err := url.Parse(discoveryURL)
		if err != nil {
			return fmt.Errorf("invalid discovery url %q: %v", discoveryURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return errors.New("discovery url must use http or https scheme")
		}

		ok := false
		for _, f := range formats {
			if f == format {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("invalid output format %q", format)
		}

		ok = false
		for _, s := range strategies {
			if s == joinStrategy {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("invalid join strategy %q", joinStrategy)
		}

		return nil
	}

	app := cli.NewApp()
	app.Name = "elastic-etcd"
	app.Usage = "auto join a cluster, either during bootstrapping or later"
	app.HideVersion = true
	app.Version = ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "discovery",
			Value:       "",
			Usage:       "a etcd discovery url",
			Destination: &discoveryURL,
			EnvVar:      "ELASTIC_ETCD_DISCOVERY",
		},
		cli.StringFlag{
			Name:        "join-strategy",
			Usage:       "the strategy to join: " + strings.Join(strategies, ", "),
			EnvVar:      "ELASTIC_ETCD_JOIN_STRATEGY",
			Value:       string(join.ReplaceStrategy),
			Destination: &joinStrategy,
		},
		cli.StringFlag{
			Name:        "data-dir",
			Usage:       "the etcd data directory",
			EnvVar:      "ELASTIC_ETCD_DATA_DIR",
			Value:       "",
			Destination: &dataDir,
		},
		cli.StringFlag{
			Name:        "o",
			Usage:       fmt.Sprintf("the output format out of: %s", strings.Join(formats, ", ")),
			Value:       "env",
			Destination: &format,
		},
		cli.StringFlag{
			Name:        "name",
			Usage:       "the cluster-unique node name",
			EnvVar:      "ELASTIC_ETCD_NAME",
			Value:       "",
			Destination: &name,
		},
		cli.IntFlag{
			Name:        "client-port",
			Usage:       "the etcd client port of all peers",
			EnvVar:      "ELASTIC_ETCD_CLIENT_PORT",
			Value:       2379,
			Destination: &clientPort,
		},
		cli.IntFlag{
			Name:        "cluster-size",
			Usage:       "the maximum etcd cluster size, default: size value of discovery url, 0 for infinit",
			EnvVar:      "ELASTIC_ETCD_CLUSTER_SIZE",
			Value:       -1,
			Destination: &clusterSize,
		},
		cli.StringFlag{
			Name:        "initial-advertise-peer-urls",
			Usage:       "the advertised peer urls of this instance",
			EnvVar:      "ELASTIC_ETCD_INITIAL_ADVERTISE_PEER_URLS",
			Value:       "http://localhost:2380",
			Destination: &initialAdvertisePeerURLs,
		},
	}
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if !strings.HasPrefix(f.Name, "test.") {
			app.Flags = append(app.Flags, cliext.FlagsFlag{Flag: f})
		}
	})

	var actionResult *EtcdConfig
	app.Action = func(c *cli.Context) error {
		glog.V(6).Infof("flags: %v", args)

		err := checkFlags()
		if err != nil {
			return err
		}

		// derive configuration values
		if dataDir == "" {
			dataDir = name + ".etcd"
		}
		fresh := true
		if fileutil.Exist(dataDir) {
			var fs []string
			fs, err = fileutil.ReadDir(dataDir)
			if err != nil {
				return err
			}
			glog.V(6).Infof("Found the following files in %s: %v", dataDir, fs)
			fresh = len(fs) == 0
		}

		jr, err := join.Join(
			discoveryURL,
			name,
			initialAdvertisePeerURLs,
			fresh,
			clientPort,
			clusterSize,
			join.Strategy(joinStrategy),
		)
		if err != nil {
			return fmt.Errorf("cluster join failed: %v", err)
		}
		actionResult = &EtcdConfig{*jr, dataDir}
		return nil
	}

	err := app.Run(args)
	if err != nil {
		return nil, "", err
	}

	return actionResult, format, nil
}
