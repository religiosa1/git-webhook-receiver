package tmpoutput_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
)

const testPipeID = "pipe-123"

// mustCreate calls Create and fatals on error.
func mustCreate(t *testing.T, mgr tmpoutput.Manager, pipeID string) io.Writer {
	t.Helper()
	w, err := mgr.Create(pipeID)
	if err != nil {
		t.Fatalf("Create(%q): %v", pipeID, err)
	}
	return w
}

// mustWrite writes s to w and fatals on error.
func mustWrite(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("Write: %v", err)
	}
}

// mustDrain calls Drain and fatals on error.
func mustDrain(t *testing.T, mgr tmpoutput.Manager, pipeID string) io.Reader {
	t.Helper()
	r, err := mgr.Drain(pipeID)
	if err != nil {
		t.Fatalf("Drain(%q): %v", pipeID, err)
	}
	return r
}

// mustReadAll reads all bytes from r and fatals on error.
func mustReadAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return b
}

func TestCreate(t *testing.T) {
	t.Run("returns a writer", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)
		if w == nil {
			t.Fatal("Create returned nil writer")
		}
	})

	t.Run("duplicate pipeID returns ErrAlreadyOpened", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		mustCreate(t, mgr, testPipeID)
		_, err := mgr.Create(testPipeID)
		if !errors.Is(err, tmpoutput.ErrAlreadyOpened) {
			t.Fatalf("want ErrAlreadyOpened, got %v", err)
		}
	})
}

func TestDrain(t *testing.T) {
	t.Run("drains written data", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)
		mustWrite(t, w, "hello world")
		r := mustDrain(t, mgr, testPipeID)
		got := mustReadAll(t, r)
		if string(got) != "hello world" {
			t.Errorf("want %q, got %q", "hello world", got)
		}
	})

	t.Run("drains empty buffer", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		mustCreate(t, mgr, testPipeID)
		r := mustDrain(t, mgr, testPipeID)
		got := mustReadAll(t, r)
		if len(got) != 0 {
			t.Errorf("want empty, got %q", got)
		}
	})

	t.Run("non-existent pipeID returns ErrNotExists", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		_, err := mgr.Drain("no-such-pipe")
		if !errors.Is(err, tmpoutput.ErrNotExists) {
			t.Fatalf("want ErrNotExists, got %v", err)
		}
	})

	t.Run("second drain returns ErrNotExists", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		mustCreate(t, mgr, testPipeID)
		mustDrain(t, mgr, testPipeID)
		_, err := mgr.Drain(testPipeID)
		if !errors.Is(err, tmpoutput.ErrNotExists) {
			t.Fatalf("want ErrNotExists, got %v", err)
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("double close is safe", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		mustCreate(t, mgr, testPipeID)
		if err := mgr.Close(testPipeID); err != nil {
			t.Fatalf("first Close: %v", err)
		}
		if err := mgr.Close(testPipeID); err != nil {
			t.Fatalf("second Close: %v", err)
		}
	})

	t.Run("close non-existent is safe", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		if err := mgr.Close("no-such-pipe"); err != nil {
			t.Fatalf("Close non-existent: %v", err)
		}
	})

	t.Run("write after close returns ErrBufferClosed", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)
		if err := mgr.Close(testPipeID); err != nil {
			t.Fatalf("Close: %v", err)
		}
		_, err := io.WriteString(w, "data")
		if !errors.Is(err, tmpoutput.ErrBufferClosed) {
			t.Fatalf("want ErrBufferClosed, got %v", err)
		}
	})
}

func TestMaxSize(t *testing.T) {
	t.Run("write within limit succeeds", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(20)
		w := mustCreate(t, mgr, testPipeID)
		mustWrite(t, w, "hello")
	})

	t.Run("write over limit returns ErrOutputTooLarge", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(5)
		w := mustCreate(t, mgr, testPipeID)
		_, err := io.WriteString(w, "hello world")
		if !errors.Is(err, tmpoutput.ErrOutputTooLarge) {
			t.Fatalf("want ErrOutputTooLarge, got %v", err)
		}
	})

	t.Run("zero max size means unlimited", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)
		mustWrite(t, w, strings.Repeat("x", 1<<20))
	})
}

func TestReader(t *testing.T) {
	t.Run("returns false for non-existent pipe", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		_, ok := mgr.Reader(context.Background(), "no-such-pipe")
		if ok {
			t.Fatal("want false, got true")
		}
	})

	t.Run("live reader sees data written after Reader call", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)

		r, ok := mgr.Reader(context.Background(), testPipeID)
		if !ok {
			t.Fatal("Reader: want true, got false")
		}

		const payload = "streamed data"
		var wg sync.WaitGroup
		wg.Go(func() {
			mustWrite(t, w, payload)
			mustDrain(t, mgr, testPipeID)
		})

		got := mustReadAll(t, r)
		wg.Wait()
		if string(got) != payload {
			t.Errorf("want %q, got %q", payload, got)
		}
	})

	t.Run("drain signals live reader with EOF", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)
		mustWrite(t, w, "pre-drain")

		r, ok := mgr.Reader(context.Background(), testPipeID)
		if !ok {
			t.Fatal("Reader: want true, got false")
		}

		mustDrain(t, mgr, testPipeID)

		got := mustReadAll(t, r)
		if string(got) != "pre-drain" {
			t.Errorf("want %q, got %q", "pre-drain", got)
		}
	})

	t.Run("close signals live reader with EOF", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		w := mustCreate(t, mgr, testPipeID)
		mustWrite(t, w, "pre-close")

		r, ok := mgr.Reader(context.Background(), testPipeID)
		if !ok {
			t.Fatal("Reader: want true, got false")
		}

		if err := mgr.Close(testPipeID); err != nil {
			t.Fatalf("Close: %v", err)
		}

		got := mustReadAll(t, r)
		if string(got) != "pre-close" {
			t.Errorf("want %q, got %q", "pre-close", got)
		}
	})

	t.Run("context cancellation unblocks live reader", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		mustCreate(t, mgr, testPipeID)

		ctx, cancel := context.WithCancel(context.Background())
		r, ok := mgr.Reader(ctx, testPipeID)
		if !ok {
			t.Fatal("Reader: want true, got false")
		}

		done := make(chan error, 1)
		go func() {
			_, err := io.ReadAll(r)
			done <- err
		}()

		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	})

	t.Run("returns false after drain", func(t *testing.T) {
		mgr := tmpoutput.NewInMemoryTmpOutput(0)
		mustCreate(t, mgr, testPipeID)
		mustDrain(t, mgr, testPipeID)
		_, ok := mgr.Reader(context.Background(), testPipeID)
		if ok {
			t.Fatal("want false after Drain, got true")
		}
	})
}

func TestConcurrentWriteRead(t *testing.T) {
	mgr := tmpoutput.NewInMemoryTmpOutput(0)
	w := mustCreate(t, mgr, testPipeID)

	r, ok := mgr.Reader(context.Background(), testPipeID)
	if !ok {
		t.Fatal("Reader: want true, got false")
	}

	const chunks = 50
	var wg sync.WaitGroup
	wg.Go(func() {
		for range chunks {
			mustWrite(t, w, "x")
		}
		mustDrain(t, mgr, testPipeID)
	})

	got := mustReadAll(t, r)
	wg.Wait()
	if len(got) != chunks {
		t.Errorf("want %d bytes, got %d", chunks, len(got))
	}
}
