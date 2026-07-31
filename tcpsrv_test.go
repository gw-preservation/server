package gw1

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func tcpAddr(ip string, port int) net.Addr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: port}
}

func TestConnLimiter_PerIPCap(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}
	addr := tcpAddr("10.0.0.1", 1000)

	for i := 0; i < maxConnsPerIP; i++ {
		assert.True(t, l.tryAcquire(addr), "acquire %d should succeed", i)
	}
	assert.False(t, l.tryAcquire(addr), "connection over the per-IP cap should be rejected")
}

func TestConnLimiter_ReleaseFreesPerIPSlot(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}
	addr := tcpAddr("10.0.0.1", 1000)

	for i := 0; i < maxConnsPerIP; i++ {
		assert.True(t, l.tryAcquire(addr))
	}
	assert.False(t, l.tryAcquire(addr))

	l.release(addr)
	assert.True(t, l.tryAcquire(addr), "releasing one connection should free a per-IP slot")
}

func TestConnLimiter_GlobalCap(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}

	for i := 0; i < maxGlobalConns; i++ {
		addr := tcpAddr(fmt.Sprintf("10.0.%d.%d", i/250, i%250), 1000+i)
		assert.True(t, l.tryAcquire(addr), "acquire %d should succeed", i)
	}
	assert.False(t, l.tryAcquire(tcpAddr("10.1.0.1", 1)), "connection over the global cap should be rejected")
}

func TestConnLimiter_ReleaseFreesGlobalSlot(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}

	for i := 0; i < maxGlobalConns; i++ {
		assert.True(t, l.tryAcquire(tcpAddr(fmt.Sprintf("10.0.%d.%d", i/250, i%250), 1000+i)))
	}
	assert.False(t, l.tryAcquire(tcpAddr("10.1.0.1", 1)))

	l.release(tcpAddr("10.0.0.0", 1000))
	assert.True(t, l.tryAcquire(tcpAddr("10.1.0.1", 1)), "releasing one connection should free a global slot")
}

func TestConnLimiter_PerIPAndGlobalIndependent(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}
	addr1 := tcpAddr("10.0.0.1", 1)

	for i := 0; i < maxConnsPerIP; i++ {
		assert.True(t, l.tryAcquire(addr1))
	}
	assert.True(t, l.tryAcquire(tcpAddr("10.0.0.2", 2)), "a different IP should still be accepted while global has room")
	assert.False(t, l.tryAcquire(addr1), "addr1 should still be at its per-IP cap")
}

func TestConnLimiter_ReleaseToZeroCleansMap(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}
	addr := tcpAddr("10.0.0.1", 1000)

	assert.True(t, l.tryAcquire(addr))
	assert.Len(t, l.byIP, 1)

	l.release(addr)
	assert.Len(t, l.byIP, 0)
	assert.Zero(t, l.total)
}

func TestConnLimiter_ReleaseNeverAcquired(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}
	l.release(tcpAddr("10.0.0.1", 1))
	assert.Zero(t, l.total)
	assert.Len(t, l.byIP, 0)
}

func TestConnLimiter_ConcurrentChurn(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}
	var wg sync.WaitGroup

	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				addr := tcpAddr(fmt.Sprintf("10.0.%d.%d", g, i%250), 1000+i)
				if l.tryAcquire(addr) {
					l.release(addr)
				}
			}
		}(g)
	}
	wg.Wait()

	assert.Zero(t, l.total)
	assert.Len(t, l.byIP, 0)
}

func TestConnLimiter_ConcurrentAcquireNoOverAllocation(t *testing.T) {
	l := connLimiter{byIP: make(map[string]int)}

	for i := 0; i < maxGlobalConns; i++ {
		assert.True(t, l.tryAcquire(tcpAddr(fmt.Sprintf("10.0.%d.%d", i/250, i%250), 1000+i)))
	}

	var wg sync.WaitGroup
	for g := 0; g < 32; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				addr := tcpAddr(fmt.Sprintf("10.1.%d.%d", g, i), 1000+i)
				assert.False(t, l.tryAcquire(addr), "acquire over the global cap must fail")
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, maxGlobalConns, l.total, "concurrent acquires must never over-allocate past the global cap")
}

func TestConnKey_StripsPort(t *testing.T) {
	assert.Equal(t, "10.0.0.1", connKey(tcpAddr("10.0.0.1", 1000)))
	assert.Equal(t, "::1", connKey(tcpAddr("::1", 1000)))
}
