package util

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer(t *testing.T) {
	testCases := []struct {
		scenario string
		fn       func(t *testing.T)
	}{
		{
			scenario: "test happy case",
			fn: func(t *testing.T) {
				ring := CreateRingBuffer[int](5)

				for i := 0; i < 20; i++ {
					item := i
					ring.Enqueue(&item)
				}

				results := make([]int, 0, 5)
				for !ring.IsEmpty() {
					item, err := ring.Dequeue()
					assert.Nil(t, err)
					results = append(results, *item)
				}

				assert.Equal(t, []int{15, 16, 17, 18, 19}, results)
			},
		},
		{
			scenario: "test race condition",
			fn: func(t *testing.T) {
				ring := CreateRingBuffer[int](10)
				lock := new(sync.Mutex)

				wg := new(sync.WaitGroup)
				wg.Add(2)
				go func() {
					defer wg.Done()

					for i := 0; i < 100000; i++ {
						item := i
						lock.Lock()
						ring.Enqueue(&item)
						fmt.Println("enqueue", item /*ring.GetSize(),*/, ring.head, ring.tail)
						lock.Unlock()
					}
				}()
				// 0 1 2 3
				// 0 1 2 3

				// 5 6 7 8 9
				go func() {
					for {
						lock.Lock()
						// size := ring.IsFull()
						if !ring.IsEmpty() {
							// fmt.Println("size", size)
							item, err := ring.Dequeue()
							assert.Nil(t, err)
							fmt.Println(*item /*ring.GetSize(),*/, ring.head, ring.tail)
							if *item == 99999 {
								fmt.Println("good: ", *item)
							}
						}
						lock.Unlock()
					}
				}()

				wg.Wait()
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.scenario, testCase.fn)
	}
}
