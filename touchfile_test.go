package touchfile

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewTouchFile(t *testing.T) {
	tf, err := NewTouchFile("")
	defer os.Remove(tf.path)
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}
	if tf == nil {
		t.Fatal("TouchFile should not be nil")
	}

	path := tf.Path()

	if path == "" {
		t.Error("TouchFile path should not be empty")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("TouchFile path should exist")
	}
}

func TestNewTouchFileWithSpecificPath(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "touchfile-test-temp-")
	if err != nil {
		t.Fatalf("(setup) failed to create temp file for named touch file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tf, err := NewTouchFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}

	if tf.Path() != tmpFile.Name() {
		t.Errorf("TouchFile path should match provided path: %s", tmpFile.Name())
	}
}

func TestLockAndUnlock(t *testing.T) {
	tf, err := NewTouchFile("")
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}
	defer os.Remove(tf.path)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = tf.SharedLock(ctx)
	if err != nil {
		t.Fatalf("should successfully acquire lock: %v", err)
	}

	err = tf.Unlock()
	if err != nil {
		t.Errorf("should successfully release lock: %v", err)
	}
}

func TestSharedLocking(t *testing.T) {
	tf1, err := NewTouchFile("")
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}
	defer os.Remove(tf1.path)

	tf2, err := NewTouchFile(tf1.Path())
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}

	tf3, err := NewTouchFile(tf1.Path())
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}

	err = tf1.SharedLock(context.Background())
	if err != nil {
		t.Fatalf("should successfully acquire first shared lock: %v", err)
	}

	err = tf2.SharedLock(context.Background())
	if err != nil {
		t.Fatalf("should successfully acquire second shared lock: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = tf3.ExclusiveLock(ctx)
	if err == nil {
		t.Error("should not be able to acquire exclusive lock, expected timeout error")
	}
}

func TestLockTimeout(t *testing.T) {
	tf1, err := NewTouchFile("")
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}
	defer os.Remove(tf1.path)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First lock the file
	err = tf1.ExclusiveLock(ctx)
	if err != nil {
		t.Fatalf("should successfully acquire initial lock: %v", err)
	}
	defer tf1.Unlock()

	// Try locking again with a different context that times out quickly
	tf2, err := NewTouchFile(tf1.path)
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer timeoutCancel()

	err = tf2.ExclusiveLock(timeoutCtx)
	if err == nil {
		t.Error("should not be able to acquire lock, expected timeout error")
	}
}

func TestWithLock(t *testing.T) {
	tf, err := NewTouchFile("")
	if err != nil {
		t.Fatalf("should not error when creating a new TouchFile: %v", err)
	}
	defer os.Remove(tf.path)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	executed := false
	err = tf.WithLock(ctx, Shared, func() error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatalf("should successfully acquire lock and execute function: %v", err)
	}

	if !executed {
		t.Fatal("provided function should have been executed")
	}
}
