package discovery

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/store"
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	"golang.org/x/net/context/ctxhttp"
)

const (
	discoveryTimeout = time.Second * 10
)

// Value reads a value from a discovery url.
func Value(ctx context.Context, baseURL, key string) (*store.Event, error) {
	ctx, _ = context.WithTimeout(ctx, discoveryTimeout)

	url := baseURL + key
	glog.V(6).Infof("Getting %s", url)
	resp, err := ctxhttp.Get(ctx, http.DefaultClient, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
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

// Delete remove a given machine from a discovery url.
func Delete(ctx context.Context, baseURL, id string) (bool, error) {
	ctx, _ = context.WithTimeout(ctx, discoveryTimeout)

	url := baseURL + "/" + strings.TrimLeft(id, "/")
	req, err := http.NewRequest("DELETE", url, strings.NewReader(""))
	if err != nil {
		return false, err
	}
	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return false, fmt.Errorf("status code %d on DELETE for %q: %s", resp.StatusCode, url, body)
	}

	return true, nil
}

// Add adds a Machine to a discovery url.
func Add(ctx context.Context, baseURL string, n *Machine) (bool, error) {
	ctx, _ = context.WithTimeout(ctx, discoveryTimeout)

	u := baseURL + "/" + n.ID
	value := strings.Join(n.NamedPeerURLs(), ",")
	data := url.Values{}
	data.Set("value", value)

	req, err := http.NewRequest("PUT", u, strings.NewReader(data.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ctxhttp.Do(ctx, http.DefaultClient, req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusConflict {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return false, fmt.Errorf("status code %d on PUT for %q: %s", resp.StatusCode, u, body)
	}

	return true, nil
}
