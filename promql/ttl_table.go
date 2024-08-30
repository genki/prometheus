package promql

import (
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/labels"
)

type TTLTable struct {
  table map[string]time.Time
  mu    sync.RWMutex
}

func NewTTLTable() *TTLTable {
  return &TTLTable{
    table: make(map[string]time.Time),
  }
}

func (c *TTLTable) Set(ls labels.Labels) {
  ttl := ls.TTL();
  if (ttl <= 0) {
    return;
  }
  time := time.Now().Add(time.Duration(ttl) * time.Second)
  c.mu.Lock()
  defer c.mu.Unlock()
  for _, l := range ls {
    t, found := c.table[l.Name]
    if found && t.After(time) {
      c.table[l.Name] = time;
    }
  }
}

func (c *TTLTable) Check(ls labels.Labels) bool {
  now := time.Now()
  c.mu.RLock()
  defer c.mu.RUnlock()
  for _, l := range ls {
    t, found := c.table[l.Name]
    if !found || t.Before(now) {return false}
  }
  return true
}

func (c *TTLTable) CleanUp() {
  c.mu.Lock()
  defer c.mu.Unlock()
  now := time.Now()
  for key, ttl := range c.table {
    if now.After(ttl) {
      delete(c.table, key)
    }
  }
}
