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

		// Protect from memory leaks: Do not return huge builders to the pool.
		// Maximum 32KB allocation retained per builder.
		if builder.Cap() <= 32*1024 {
		stringsBuildersPool.Put(builder)
		}

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

// Is reports whether any error in b's buffer matches target.
func (b *BufferedError) Is(target error) bool {
	b.RLock()
	defer b.RUnlock()
	for _, err := range b.errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As finds the first error in b's buffer that matches target, and if one is found, sets
// target to that error value and returns true. Otherwise, it returns false.
func (b *BufferedError) As(target any) bool {
	b.RLock()
	defer b.RUnlock()
	for _, err := range b.errors {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}

func (b *BufferedError) Grow(n int) {
	b.Lock()
	defer b.Unlock()
	if cap(b.errors) < n {
		b.grow(n)
	}
}

func (b *BufferedError) grow(n int) {
		old := b.errors
		b.errors = make([]error, len(old), n)
		copy(b.errors, old)
}

// Clear clears internal slice of errors.
// Freed memory will be collected by GC.
func (b *BufferedError) Clear() {
	b.Lock()
	b.errors = ([]error)(nil)
	b.Unlock()
}

// Add adds given error to the errors buffer.
// No-op if error is nil.
func (b *BufferedError) Add(err error) {
	if err == nil {
		return
	}

	b.Lock()
	b.errors = append(b.errors, err)
	b.Unlock()
}

// Err returns nil if error buffer is empty.
func (b *BufferedError) Err() error {
	b.RLock()
	defer b.RUnlock()

	if len(b.errors) == 0 {
		return nil
	}

	return b
}

// New creates empty [BufferedError].
func New() *BufferedError {
	return &BufferedError{errors: ([]error)(nil)}
}

// NewFromError creates new [BufferedError] from the given error.
func NewFromError(err error) *BufferedError {
	if err == nil {
		return New()
	}
	return &BufferedError{errors: []error{err}}
}
