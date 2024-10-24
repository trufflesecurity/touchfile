// Package touchfile provides a mechanism to create, lock, and manage a touch
// file for coordinating access between different processes.
//
// Mutexes are used to coordinate between goroutines. Touch files are used to
// coordinate between different processes. Because touch files aren't truly
// atomic, this package uses flock to acquire a voluntary lock on the file.
//
// This package creates a temporary file (unless otherwise specified) to use as
// a lock file. A voluntary lock is acquired on the file using the flock system
// call.
//
// WARNING: Like all go code related to concurrency, this module is NOT
// reentrant. And because go doesn't have a way to detect reentrancy, it's up
// to the caller to avoid deadlocks caused by concurrent access to this module.
//
// WARNING: Because flock is used, this module may not be compatible with all
// file systems (notably networked file systems like NFS).
//
// Example usage:
//
// Basic usage:
//
//  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//  defer cancel()
//
//  tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
//  if err != nil {
//      log.Fatalf("failed to create touch file: %v", err)
//  }
//
//  if err := tf.Lock(ctx); err != nil {
//      log.Fatalf("failed to acquire lock: %v", err)
//  }
//  defer func() {
//      if err := tf.Unlock(); err != nil {
//          log.Printf("failed to release lock: %v", err)
//      }
//  }()
//
// Global lock using the program binary:
//
//  program, err := os.Executable()
//  if err != nil {
//    log.Fatalf("failed to determine executable path: %v", err)
//  }
//
//  tf, err := touchfile.NewTouchFile(program)
//  ...
//
// Using WithLock for a critical section:
//
//  tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
//  if err != nil {
//      log.Fatalf("failed to create touch file: %v", err)
//  }
//
//  err = tf.WithLock(ctx, func() error {
//      // Critical section
//      fmt.Println("Doing some work while holding the lock")
//      return nil
//  })
//  if err != nil {
//      log.Fatalf("operation failed: %v", err)
//  }
package touchfile

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// TouchFile struct holds the path to the touch file and a flock instance for
// advisory locking. A mutex is included to ensure safe access across multiple
// goroutines.
type TouchFile struct {
	mu   sync.Mutex
	lock *flock.Flock
	path string
}

// LockType is an enum for the type of lock to acquire on the touch file.
type LockType int

const (
	// An Exclusive lock prevents any other processes or threads from acquiring
	// any type of lock on the touch file.
	Exclusive LockType = iota

	// A Shared lock allows multiple processes or threads to acquire another
	// Shared lock on the touch file, while preventing an Exclusive lock.
	Shared
)

const lockRetryInterval = 10 * time.Millisecond


// NewTouchFile creates a TouchFile using flock for advisory locking. If the
// provided path is empty, a temporary file path will be used instead. The path
// is converted to an absolute path.
//
// Example:
//
//  tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
//  if err != nil {
//      log.Fatalf("failed to create touch file: %v", err)
//  }
func NewTouchFile(path string) (*TouchFile, error) {
	absPath, err := createTouchFile(path)
	if err != nil {
		return nil, err
	}

	return &TouchFile{
		mu:   sync.Mutex{},
		lock: flock.New(absPath),
		path: absPath,
	}, nil
}

func createTouchFile(path string) (string, error) {
	if path == "" {
		tmpFile, err := os.CreateTemp("", "touchfile-")
		if err != nil {
			return "", err
		}

		path = tmpFile.Name()
		tmpFile.Close()
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("unable to determine absolute path: %v", err)
	}

	// Verify that the parent directory exists
	dir := filepath.Dir(absPath)
	_, err = os.Stat(dir)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("parent directory does not exist: %s", dir)
	}

	return absPath, nil
}

// Path returns the path to the touch file.
func (tf *TouchFile) Path() string {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	return tf.path
}

// Lock attempts to acquire the file lock. It will keep attempting to acquire
// the lock until the provided context times out or is canceled. If the lock
// cannot be acquired within the context's deadline, an error is returned.
//
// Example:
//
//  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//  defer cancel()
//
//  tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
//  if err != nil {
//      log.Fatalf("failed to create touch file: %v", err)
//  }
//
//  if err := tf.Lock(ctx, touchfile.Shared); err != nil {
//      log.Fatalf("failed to acquire shared lock: %v", err)
//  }
//  defer func() {
//      if err := tf.Unlock(); err != nil {
//          log.Printf("failed to release lock: %v", err)
//      }
//  }()
func (tf *TouchFile) Lock(ctx context.Context, lockType LockType) error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	var locked bool
	var err error

	switch lockType {
	case Shared:
		locked, err = tf.lockShared(ctx)
	case Exclusive:
		locked, err = tf.lockExclusive(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to acquire lock on file %s: %w", tf.path, err)
	}
	if !locked {
		return fmt.Errorf("unable to acquire lock on file %s within context timeout", tf.path)
	}

	return nil
}

// SharedLock attempts to acquire a Shared lock on the touch file.
func (tf *TouchFile) SharedLock(ctx context.Context) error {
	return tf.Lock(ctx, Shared)
}

// ExclusiveLock attempts to acquire an Exclusive lock on the touch file.
func (tf *TouchFile) ExclusiveLock(ctx context.Context) error {
	return tf.Lock(ctx, Exclusive)
}

func (tf *TouchFile) lockShared(ctx context.Context) (bool, error) {
	return tf.lock.TryRLockContext(ctx, lockRetryInterval)
}

func (tf *TouchFile) lockExclusive(ctx context.Context) (bool, error) {
	return tf.lock.TryLockContext(ctx, lockRetryInterval)
}

// Unlock releases the lock on the touch file. If an error occurs during
// unlocking, it is returned to the caller.
//
// Example:
//
//  tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
//  if err != nil {
//      log.Fatalf("failed to create touch file: %v", err)
//  }
//
//  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//  defer cancel()
//
//  if err := tf.Lock(ctx); err != nil {
//      log.Fatalf("failed to acquire lock: %v", err)
//  }
//
//  if err := tf.Unlock(); err != nil {
//      log.Printf("failed to release lock: %v", err)
//  }
func (tf *TouchFile) Unlock() error {
	tf.mu.Lock()
	defer tf.mu.Unlock()

	if err := tf.lock.Unlock(); err != nil {
		return fmt.Errorf("failed to unlock file at %s: %w", tf.path, err)
	}
	return nil
}

// WithLock is a convenience function that locks the touch file, executes the
// provided function, and then unlocks the touch file. If the lock cannot be
// acquired, the function will return an error. The lock is always released
// after the function is executed, even if the function returns an error.
//
// Example:
//
//  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//  defer cancel()
//
//  tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
//  if err != nil {
//      log.Fatalf("failed to create touch file: %v", err)
//  }
//
//  err = tf.WithLock(ctx, func() error {
//      fmt.Println("Doing some work while holding the lock")
//      return nil
//  })
//  if err != nil {
//      log.Fatalf("operation failed: %v", err)
//  }
func (tf *TouchFile) WithLock(ctx context.Context, lockType LockType, f func() error) error {
	if err := tf.Lock(ctx, lockType); err != nil {
		return err
	}

	defer func() {
		if err := tf.Unlock(); err != nil {
			log.Printf("failed to unlock touch file at %s: %v\n", tf.path, err)
		}
	}()

	return f()
}
