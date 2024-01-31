package readermanager

import (
	"fmt"
)

func defaultErrorHandleFn(err error) {
	fmt.Println("get error: ", err) // TODO
}

type readerManagerConfig struct {
	readerManagerOptions []readerManagerOption

	partitionBindObserverReaderManagerOptions []partitionBindObserverReaderManagerOption
	groupIDReaderManagerOptions               []groupIDReaderManagerOption
	specPartitionReaderManagers               []specPartitionReaderManagerOption

	readerOptions []readerOption
}

type readerManagerOption func(*readerManager)

func AddErrorHandleFn(fn func(err error)) ReaderManagerConfigOption {
	return func(rmc *readerManagerConfig) {
		rmc.readerManagerOptions = append(rmc.readerManagerOptions, func(rm *readerManager) {
			rm.errorHandleFn = fn
		})
		rmc.readerOptions = append(rmc.readerOptions, func(r *Reader) {
			r.errorHandleFn = fn
		})
	}
}

type ReaderManagerConfigOption func(*readerManagerConfig)

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