//go:build ignore

package actionsdb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
)

var (
	errAlreadyOpened  = errors.New("output buffer for the file already opened")
	errNotExists      = errors.New("output buffer doesn't exist or was closed")
	errOutputTooLarge = errors.New("output buffer exceeded maximum size")
	errBufferClosed   = errors.New("output buffer is closed")
)

type tmpOutputManager interface {
	Open(pipeID string) (io.Writer, error)                      // creates a temp writer for action output
	Has(pipeID string) bool                                     // Has checks if there's an open sink
	Read(ctx context.Context, pipeID string) (io.Reader, error) // ongoing-aware live reader
	Drain(pipeID string) (io.Reader, error)                     // mark done, return snapshot for DB persistence
}

var _ tmpOutputManager = (*inMemoryTmpOutput)(nil)

// liveBuffer is a concurrent-safe append-only buffer. Writers append; readers
// each track their own offset and block until new data arrives or done is set.
type liveBuffer struct {
	mu      sync.Mutex
	data    []byte
	done    bool
	notify  chan struct{} // closed on each write or close to wake blocked readers
	maxSize int64         // 0 means unlimited
}

func newLiveBuffer(maxSize int64) *liveBuffer {
	return &liveBuffer{notify: make(chan struct{}), maxSize: maxSize}
}

func (b *liveBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	if b.done {
		b.mu.Unlock()
		return 0, errBufferClosed
	}
	if b.maxSize > 0 && int64(len(b.data)+len(p)) > b.maxSize {
		b.mu.Unlock()
		return 0, errOutputTooLarge
	}
	b.data = append(b.data, p...)
	old := b.notify
	b.notify = make(chan struct{})
	b.mu.Unlock()
	close(old)
	return len(p), nil
}

func (b *liveBuffer) closeAndSnapshot() []byte {
	b.mu.Lock()
	snapshot := make([]byte, len(b.data))
	copy(snapshot, b.data)
	old := b.notify
	b.done = true
	b.mu.Unlock()
	close(old)
	return snapshot
}

// liveReader is an io.Reader with its own read offset into a liveBuffer.
// Blocks on Read until new data arrives, EOF (buffer closed), or ctx cancelled.
type liveReader struct {
	buf    *liveBuffer
	ctx    context.Context
	offset int
}

func (r *liveReader) Read(p []byte) (int, error) {
	for {
		r.buf.mu.Lock()
		if r.offset < len(r.buf.data) {
			n := copy(p, r.buf.data[r.offset:])
			r.offset += n
			r.buf.mu.Unlock()
			return n, nil
		}
		if r.buf.done {
			r.buf.mu.Unlock()
			return 0, io.EOF
		}
		notify := r.buf.notify
		r.buf.mu.Unlock()

		select {
		case <-notify:
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		}
	}
}

type inMemoryTmpOutput struct {
	buffers map[string]*liveBuffer
	mutex   sync.Mutex
	maxSize int64
}

func newInMemoryTmpOutput(maxSize int64) *inMemoryTmpOutput {
	return &inMemoryTmpOutput{buffers: make(map[string]*liveBuffer), maxSize: maxSize}
}

// Drain implements [tmpOutputManager].
func (i *inMemoryTmpOutput) Drain(pipeID string) (io.Reader, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	buffer, ok := i.buffers[pipeID]
	if !ok {
		return nil, errNotExists
	}
	delete(i.buffers, pipeID)

	return bytes.NewReader(buffer.closeAndSnapshot()), nil
}

// Has implements [tmpOutputManager].
func (i *inMemoryTmpOutput) Has(pipeID string) bool {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	_, ok := i.buffers[pipeID]
	return ok
}

// Open implements [tmpOutputManager].
func (i *inMemoryTmpOutput) Open(pipeID string) (io.Writer, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if _, ok := i.buffers[pipeID]; ok {
		return nil, errAlreadyOpened
	}
	buf := newLiveBuffer(i.maxSize)
	i.buffers[pipeID] = buf
	return buf, nil
}

// Read implements [tmpOutputManager].
func (i *inMemoryTmpOutput) Read(ctx context.Context, pipeID string) (io.Reader, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	buffer, ok := i.buffers[pipeID]
	if !ok {
		return nil, errNotExists
	}

	return &liveReader{buf: buffer, ctx: ctx}, nil
}
