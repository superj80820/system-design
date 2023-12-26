package memory

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryConfigRepo(t *testing.T) {
	memoryConfigRepo := CreateMemoryConfigRepo[int](0)
	wg := new(sync.WaitGroup)
	wg.Add(10000)
	for i := 0; i < 10000; i++ {
		go func() {
			defer wg.Done()

			var done bool
			for !done {
				old := memoryConfigRepo.Get()
				new := old + 1
				done = memoryConfigRepo.CAS(old, new)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 10000, memoryConfigRepo.Get())
}
