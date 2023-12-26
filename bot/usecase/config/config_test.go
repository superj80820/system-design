package memory

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/superj80820/system-design/config/repository/memory"
)

func TestMemoryConfigService(t *testing.T) {
	memoryConfigService := CreateMemoryUseCase[int](memory.CreateMemoryConfigRepo(0))
	wg := new(sync.WaitGroup)
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func() {
			defer wg.Done()

			var done bool
			for !done {
				old := memoryConfigService.Get()
				new := old + 1
				done = memoryConfigService.CAS(old, new)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 1000, memoryConfigService.Get())
}
