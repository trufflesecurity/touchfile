[![test suite](https://github.com/trufflesecurity/touchfile/actions/workflows/run-tests.yml/badge.svg)](https://github.com/trufflesecurity/touchfile/actions/workflows/run-tests.yml)

# touchfile
`touchfile` is a Go package that provides a mechanism for creating, locking,
and managing touch files to coordinate access between different processes and
goroutines. It uses file locks (`flock`) to ensure only one process or thread has
exclusive access to a critical section at a time.

This package is designed for scenarios where multiple processes or threads need
to coordinate on a shared resource (e.g., a file). It leverages mutexes for
goroutine safety and file-based locks for process-level coordination.

## Features
- **Mutex protection:** Ensures safe access across multiple goroutines.
- **Process-level locking:** Uses advisory locking (`flock`) for file-level coordination between different processes.
- **Support for both shared and exclusive locks:** Choose between shared (read) locks and exclusive (write) locks.
- **Timeout handling with context:** Automatically handle lock acquisition with context-based timeouts and cancellation.
- **Critical section helpers:** Simplified `WithLock` function for running code within a lock-protected critical section.

## Important notes

### Non-reentrant
This package is **not reentrant**. It is up to the caller to avoid recursive
calls that attempt to acquire the same lock, which would result in deadlocks.

### File system compatibility
`flock` may not be compatible with all file systems (notably networked file
systems like NFS). Please ensure your file system supports advisory file locks.

## Installation
To install the package, run:

```bash
go get github.com/trufflesecurity/touchfile
```

In your Go module, import the package with:

```go
import "github.com/trufflesecurity/touchfile"
```

## Example usage

### Basic usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
if err != nil {
    log.Fatalf("failed to create touch file: %v", err)
}

if err := tf.Lock(ctx); err != nil {
    log.Fatalf("failed to acquire lock: %v", err)
}
defer func() {
    if err := tf.Unlock(); err != nil {
        log.Printf("failed to release lock: %v", err)
    }
}()
```

### Global lock using the program binary

```go
program, err := os.Executable()
if err != nil {
  log.Fatalf("failed to determine executable path: %v", err)
}

tf, err := touchfile.NewTouchFile(program)
...
```

### Using WithLock for a critical section

```go
tf, err := touchfile.NewTouchFile("/tmp/mylockfile")
if err != nil {
    log.Fatalf("failed to create touch file: %v", err)
}

err = tf.WithLock(ctx, func() error {
    fmt.Println("Doing some work while holding the lock")
    return nil
})
if err != nil {
    log.Fatalf("operation failed: %v", err)
}
```

## API
### `NewTouchFile(path string) (*TouchFile, error)`
Creates a new TouchFile instance using the specified file path. If the path is
empty, a temporary file will be used.

### `Lock(ctx context.Context, lockType LockType) error`
Acquires the specified lock (Shared or Exclusive) on the touch file. The
function will attempt to acquire the lock until the context times out or is
canceled.

### `Unlock() error`
Releases the lock on the touch file.

### `WithLock(ctx context.Context, lockType LockType, f func() error) error`
A convenience function that locks the touch file, executes the provided
function, and then releases the lock. It ensures the lock is always released,
even if the function returns an error.
