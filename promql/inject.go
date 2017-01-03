package promql

import (
	docker "github.com/fsouza/go-dockerclient"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"net"
	"strings"
	"time"
)

type Cache struct {
	key  string
	lset model.LabelSet
	time time.Time
}

var client, _ = docker.NewClient("unix:///var/run/docker.sock")
var cache = map[string]Cache{}

func init() {
	if client != nil && client.Ping() == nil {
		log.Infof("docker swarm injection enabled.")
	} else {
		log.Infof("docker swarm injection disabled.")
		client = nil
	}
}

func injectMetrics(matrix model.Matrix) {
	for _, ss := range matrix {
		instance, ok := ss.Metric[model.InstanceLabel]
		if !ok {
			continue
		}

		host, _, err := net.SplitHostPort(string(instance))
		if err != nil {
			continue
		}
		if cached, ok := cache[host]; ok {
			if time.Since(cached.time).Seconds() < 5.0 {
				ss.Metric = model.Metric(cached.lset.Merge(model.LabelSet(ss.Metric)))
				continue
			}
		}

		names, err := net.LookupAddr(host)
		if err != nil || len(names) == 0 {
			continue
		}
		domain := names[0]

		if cached, ok := cache[host]; ok {
			__domain, ok := cached.lset["__domain"]
			if ok && domain == string(__domain) {
				cached.time = time.Now()
				ss.Metric = model.Metric(cached.lset.Merge(model.LabelSet(ss.Metric)))
				continue
			} else {
				delete(cache, cached.key)
			}
		}

		// get the domain name
		lset := model.LabelSet{
			"__domain": model.LabelValue(domain),
		}

		// find hostname by using docker API
		if client != nil {
			items := strings.Split(domain, ".")
			task := strings.Join(items[:2], ".")
			cname := strings.Join(items[:3], ".")
			lset = lset.Merge(model.LabelSet{
				"__service":   model.LabelValue(items[0]),
				"__task":      model.LabelValue(task),
				"__container": model.LabelValue(cname),
			})
			if c, err := client.InspectContainer(cname); err == nil {
				if nid, ok := c.Config.Labels["com.docker.swarm.node.id"]; ok {
					if node, err := client.InspectNode(nid); err == nil {
						lset = lset.Merge(model.LabelSet{
							"__host": model.LabelValue(node.Description.Hostname),
						})
					}
				}
			}
		}

		// Original metric remains if name collided.
		cache[host] = Cache{key: host, lset: lset, time: time.Now()}
		ss.Metric = model.Metric(lset.Merge(model.LabelSet(ss.Metric)))
	}
}
