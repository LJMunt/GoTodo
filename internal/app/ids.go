package app

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

func NewULID() string {
	mu.Lock()
	defer mu.Unlock()

	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
