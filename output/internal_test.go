package output

import (
	"sync"
	"testing"
	"time"

	"github.com/FreifunkBremen/yanic/runtime"
	"github.com/stretchr/testify/assert"
)

type testConn struct {
	Output
	countSave int
	sync.Mutex
	sync.WaitGroup
}

func (c *testConn) Save(nodes *runtime.Nodes) {
	c.Lock()
	c.countSave++
	c.Unlock()
	c.Done()
}
func (c *testConn) Get() int {
	c.Lock()
	defer c.Unlock()
	return c.countSave
}

func TestStart(t *testing.T) {
	assert := assert.New(t)

	conn := &testConn{}
	config := &runtime.Config{
		Nodes: struct {
			StatePath    string           `toml:"state_path"`
			SaveInterval runtime.Duration `toml:"save_interval"`
			OfflineAfter runtime.Duration `toml:"offline_after"`
			PruneAfter   runtime.Duration `toml:"prune_after"`
			Output       map[string]interface{}
		}{
			SaveInterval: runtime.Duration{Duration: time.Millisecond * 10},
		},
	}
	assert.Nil(quit)

	Start(conn, nil, config)
	assert.NotNil(quit)

	assert.Equal(0, conn.Get())
	conn.Add(1)
	conn.Wait()
	assert.Equal(1, conn.Get())

	conn.Add(1)
	conn.Wait()
	Close()
	assert.Equal(2, conn.Get())

}
