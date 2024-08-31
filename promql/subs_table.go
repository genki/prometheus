package promql

import (
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/model/labels"
)

// subscription
type Sub struct {
  denom int
  time time.Time
}

type SubCount struct {
  Sub
  actives int
}

type SubsTable struct {
  table map[string]*map[uint64]Sub
  logger log.Logger
  mu    sync.RWMutex
  count int
  defaultTTL int
}

func NewSubsTable(logger log.Logger, defaultTTL int) *SubsTable {
  if defaultTTL <= 0 {
    // int max
    defaultTTL = 1<<31 - 1
  }
  level.Info(logger).Log(
    "msg", "creating new subs table", "default TTL", defaultTTL)
  return &SubsTable{
    table: make(map[string]*map[uint64]Sub),
    logger: logger,
    defaultTTL: defaultTTL,
  }
}

func (c *SubsTable) take(key string) *map[uint64]Sub {
  subs, found := c.table[key]
  if found {return subs}
  subs1 := make(map[uint64]Sub)
  c.table[key] = &subs1
  return &subs1
}

func (c *SubsTable) Set(ls labels.Labels, ttl int) {
  hash := ls.Hash()
  time := time.Now().Add(time.Duration(ttl) * time.Second)
  sub := Sub{
    denom: len(ls),
    time: time,
  }
  c.mu.Lock()
  defer c.mu.Unlock()
  for _, l := range ls {
    name := l.Name
    if name == "__name__" { name = l.Value }
    subs := c.take(name)
    sub0, found := (*subs)[hash]
    if !found || sub0.time.Before(time) {
      (*subs)[hash] = sub
    }
  }
}

func (c *SubsTable) Check(ls labels.Labels) bool {
  now := time.Now()
  c.mu.RLock()
  defer c.mu.RUnlock()
  defer func() {
    c.count++
    if (c.count > 10000) {
      c.count = 0
      go c.CleanUp()
    }
  }()

  table := make(map[uint64]SubCount)
  for _, l := range ls {
    name := l.Name
    if name == "__name__" { name = l.Value }
    subs, found := c.table[name]
    if !found {continue}
    for hash, sub := range *subs {
      if now.After(sub.time) {continue}
      subc, found := table[hash]
      if found {
        subc.actives++
      } else {
        table[hash] = SubCount{
          Sub: sub,
          actives: 1,
        }
      }
    }
  }
  for _, subc := range table {
    if subc.actives >= subc.denom {
      return true
    }
  }
  return false
}

func (c *SubsTable) CleanUp() {
  var (
    count int = 0
    deleted int = 0
  )
  c.mu.Lock()
  defer c.mu.Unlock()
  now := time.Now()
  for name, subs := range c.table {
    for hash, sub := range *subs {
      if now.After(sub.time) {
        delete(*subs, hash)
        deleted++
      }
      count++
    }
    if len(*subs) == 0 {
      delete(c.table, name)
    }
  }
  level.Info(c.logger).Log(
    "msg", "cleaned up ttl table", "count", count, "deleted", deleted)
}
