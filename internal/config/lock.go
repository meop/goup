package config

import (
	"github.com/meop/golie/lock"

	"github.com/meop/goup/internal/store"
)

// AcquireLock takes goup's lock over ~/.goup and returns the function that
// releases it.
func AcquireLock() (func(), error) {
	l, err := store.Layout()
	if err != nil {
		return nil, err
	}
	return lock.Acquire(l.LockPath(), "goup")
}
