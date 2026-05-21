package tmpoutput

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
)

var (
	ErrAlreadyOpened  = errors.New("output buffer for the file already opened")
	ErrNotExists      = errors.New("output buffer doesn't exist or was closed")
	ErrOutputTooLarge = errors.New("output buffer exceeded maximum size")
	ErrBufferClosed   = errors.New("output buffer is closed")
)

type Manager interface {
	Create(pipeID string) (io.Writer, error)                     // creates a temp writer for action output
	Drain(pipeID string) (io.Reader, error)                      // closes the writer and returns the final reader
	Close(pipeID string) error                                   // close the buffer, double close is a safe operation and does nothing
	Reader(ctx context.Context, pipeID string) (io.Reader, bool) // ongoing-aware live reader
}

var _ Manager = (*inMemoryTmpOutput)(nil)

// liveBuffer is a concurrent-safe append-only buffer. Writers append; readers
// each track their own offset and block until new data arrives or done is set.
type liveBuffer struct {
	mu      sync.RWMutex
	data    []byte
	done    bool
	notify  chan struct{} // closed on each write or close to wake blocked readers
	maxSize int           // 0 means unlimited
}

func newLiveBuffer(maxSize int) *liveBuffer {
	return &liveBuffer{notify: make(chan struct{}), maxSize: maxSize}
}

func (b *liveBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.done {
		return 0, ErrBufferClosed
	}
	if b.maxSize > 0 && len(b.data)+len(p) > b.maxSize {
		return 0, ErrOutputTooLarge
	}
	b.data = append(b.data, p...)
	old := b.notify
	b.notify = make(chan struct{})
	close(old)
	return len(p), nil
}

func (b *liveBuffer) close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.done = true
	close(b.notify)
}

func (b *liveBuffer) drain() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	snapshot := make([]byte, len(b.data))
	copy(snapshot, b.data)
	old := b.notify
	b.done = true
	close(old)
	return snapshot
}

// liveReader is an io.Reader with its own read offset into a liveBuffer.
// Blocks on Read until new data arrives, EOF (buffer closed), or ctx canceled.
type liveReader struct {
	buf    *liveBuffer
	ctx    context.Context
	offset int
}

func (r *liveReader) Read(p []byte) (int, error) {
	for {
		r.buf.mu.RLock()
		if r.offset < len(r.buf.data) {
			n := copy(p, r.buf.data[r.offset:])
			r.offset += n
			r.buf.mu.RUnlock()
			return n, nil
		}
		if r.buf.done {
			r.buf.mu.RUnlock()
			return 0, io.EOF
		}
		notify := r.buf.notify
		r.buf.mu.RUnlock()

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
	maxSize int
}

func NewInMemoryTmpOutput(maxSize int) *inMemoryTmpOutput {
	return &inMemoryTmpOutput{buffers: make(map[string]*liveBuffer), maxSize: maxSize}
}

// Create implements [Manager].
func (i *inMemoryTmpOutput) Create(pipeID string) (io.Writer, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if _, ok := i.buffers[pipeID]; ok {
		return nil, ErrAlreadyOpened
	}
	buf := newLiveBuffer(i.maxSize)
	i.buffers[pipeID] = buf
	return buf, nil
}

// Close implements [Manager].
func (i *inMemoryTmpOutput) Close(pipeID string) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	buffer, ok := i.buffers[pipeID]
	if !ok || buffer.done {
		return nil
	}

	buffer.close()
	delete(i.buffers, pipeID)
	return nil
}

// Drain implements [Manager].
func (i *inMemoryTmpOutput) Drain(pipeID string) (io.Reader, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	buffer, ok := i.buffers[pipeID]
	if !ok {
		return nil, ErrNotExists
	}
	delete(i.buffers, pipeID)

	return bytes.NewReader(buffer.drain()), nil
}

// Reader implements [Manager].
func (i *inMemoryTmpOutput) Reader(ctx context.Context, pipeID string) (io.Reader, bool) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	buffer, ok := i.buffers[pipeID]
	if !ok {
		return nil, false
	}

	return &liveReader{buf: buffer, ctx: ctx}, true
}
