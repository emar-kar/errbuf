// Package errbuf allows to bufferize errors before processing them.
package errbuf

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

var (
	// Internal pool to reduce GC work.
	stringsBuildersPool = sync.Pool{
		New: func() any { return &strings.Builder{} },
	}

	singleLineSep = []byte("; ")
	multilineSep  = []byte("\n")
)

// BufferedError wrapper around errors slice with mutex.
type BufferedError struct {
	sync.RWMutex

	errors []error
}

// Error formats errors in the buffer into the string.
func (b *BufferedError) Error() string {
	b.RLock()
	defer b.RUnlock()

	switch len(b.errors) {
	case 0:
		return ""
	case 1:
		return b.errors[0].Error()
	default:
		size := (len(b.errors) - 1) * len(singleLineSep)

		for _, err := range b.errors {
			size += len(err.Error())
		}

		builder := stringsBuildersPool.Get().(*strings.Builder)
		builder.Reset()
		builder.Grow(size)

		b.writeSingleLine(builder)

		result := builder.String()
		stringsBuildersPool.Put(builder)
		return result
	}
}

func (b *BufferedError) writeSingleLine(w io.Writer) {
	for i, err := range b.errors {
		if i > 0 {
			w.Write(singleLineSep)
		}

		io.WriteString(w, err.Error())
	}
}

func (b *BufferedError) writeMultiLine(w io.Writer, ident int) {
	spaces := make([]byte, ident)
	for i := range ident {
		spaces[i] = ' '
	}

	for i, err := range b.errors {
		if i > 0 {
			w.Write(multilineSep)
		}

		if e, ok := err.(*BufferedError); ok {
			e.writeMultiLine(w, ident+1)
		} else {
			w.Write(spaces)
			io.WriteString(w, err.Error())
		}
	}
}

func (b *BufferedError) Format(f fmt.State, verb rune) {
	b.RLock()
	defer b.RUnlock()

	switch verb {
	case 'v':
		if f.Flag('+') {
			fmt.Fprintf(f, "BufferedError{errors:%+v}", b.errors)
			return
		}

		b.writeMultiLine(f, 0)
	default:
		b.writeSingleLine(f)
	}
}

// Unwrap returns a copy of the buffer errors.
func (b *BufferedError) Unwrap() []error {
	b.RLock()
	defer b.RUnlock()

	return append(([]error)(nil), b.errors...)
}

// Grow grows internal errors slice capacity to hold n number of errors.
// If actual capacity is bigger than n this function is no-op.
func (b *BufferedError) Grow(n int) {
	b.RLock()
	if cap(b.errors) < n {
		old := b.errors
		b.errors = make([]error, len(old), n)
		copy(b.errors, old)
	}
	b.RUnlock()
}

// Clear clears internal slice of errors.
// Freed memory will be collected by GC.
func (b *BufferedError) Clear() {
	b.Lock()
	b.errors = ([]error)(nil)
	b.Unlock()
}

// Add adds given error to the errors buffer.
// No-op if errors len is 0.
func (b *BufferedError) Add(err error) {
	if err == nil {
		return
	}

	b.Lock()
	b.errors = append(b.errors, err)
	b.Unlock()
}

// Err returns nil if error buffer is empty or contains
// only nil errors.
func (b *BufferedError) Err() error {
	b.Lock()
	defer b.Unlock()

	if len(b.errors) == 0 {
		return nil
	}

	return b
}

// NewErrorsBuffer creates empty [BufferedError].
//
// Deprecated: use [New] instead.
func NewErrorsBuffer() *BufferedError {
	return New()
}

// NewBufferFromError creates new [BufferedError] from the given error.
//
// Deprecated: use [NewFromError] instead.
func NewBufferFromError(err error) *BufferedError {
	return NewFromError(err)
}

// New creates empty [BufferedError].
func New() *BufferedError {
	return &BufferedError{errors: ([]error)(nil)}
}

// NewFromError creates new [BufferedError] from the given error.
func NewFromError(err error) *BufferedError {
	return &BufferedError{errors: []error{err}}
}

// NewBufferFromWarning creates new [BufferedError] and adds given error to warnings.
//
// Deprecated: warnings buffer was removed.
func NewBufferFromWarning(err error) *BufferedError {
	panic("BufferedError does not contain warnings buffer")
}
