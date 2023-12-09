package readermanager

import (
	"context"
	"fmt"
)

type ReaderManager interface {
	AddObserver(observer *Observer) bool
	SyncStartConsume(ctx context.Context) bool
	StartConsume(ctx context.Context) bool
	StopConsume() bool
	RemoveObserverWithHook(observer *Observer) bool
	IfNoObserversThenStopConsume()
	GetObserversLen() int
}

func defaultErrorHandleFn(err error) {
	fmt.Println("get error: ", err) // TODO
}

type readerManagerConfig struct {
	readerManagerOptions                      []readerManagerOption
	partitionBindObserverReaderManagerOptions []partitionBindObserverReaderManagerOption
	groupIDReaderManagerOptions               []groupIDReaderManagerOption
	specPartitionReaderManagers               []specPartitionReaderManagerOption

	readerOptions []readerOption
}

type readerManagerOption func(*readerManager)

func AddErrorHandleFn(fn func(err error)) readerManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerManagerOptions = append(rmc.readerManagerOptions, func(rm *readerManager) {
			rm.errorHandleFn = fn
		})
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) {
			r.errorHandleFn = fn
		})
	}
}

type readerManagerConfigOption func(*readerManagerConfig)

type readerManager struct {
	errorHandleFn func(err error)
}

func createReaderManager(options ...readerManagerOption) *readerManager {
	rm := &readerManager{
		errorHandleFn: defaultErrorHandleFn,
	}

	for _, option := range options {
		option(rm)
	}

	return rm
}
