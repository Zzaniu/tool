package distributed_lock

import (
    "errors"
)

var (
    LockOccupied = errors.New("the lock has been occupied")
    LockTimeout  = errors.New("the lock timeout")
)

type DistributedLock interface {
    Lock(string, int) error
    UnLock() error
}
