// Package errbuf provides a thread-safe buffer for accumulating multiple errors.
// It implements the standard error interface and supports custom formatting
// for single-line or multi-line display of the aggregated errors.
package errbuf

import (
	"errors"
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

// BufferedError represents an aggregated collection of errors.
// It is safe for concurrent use by multiple goroutines. The zero value
// is an empty, ready-to-use error buffer.
type BufferedError struct {
	sync.RWMutex

	errors []error
}

// Error returns a string representation of all buffered errors.
// If the buffer is empty, it returns an empty string.
// If the buffer contains exactly one error, it returns that error's string representation.
// For two or more errors, they are joined by a semicolon separator ("; ").
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

		w.Write(spaces)
		io.WriteString(w, err.Error())
	}
}

// Format implements the fmt.Formatter interface to support custom error formatting.
//
// The following format verbs are supported:
//
//	%s or %v   Prints errors continuously separated by semicolons (e.g. "err1; err2").
//	%v         When not used with the '+' flag, it acts the same as a multiline printer
//	           separating each error by newline and indenting nested BufferedErrors.
//	%+v        Prints the raw internal representation: "BufferedError{errors:[...]}".
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

// Unwrap returns a shallow copy of the underlying slice of accumulated errors.
// Modifications to the returned slice will not affect the buffer.
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

// Grow increases the internal slice capacity to accommodate n errors.
// If the internal capacity is already greater than or equal to n, this is a no-op.
// This is useful for optimizing allocations when the number of errors to be added
// is known in advance.
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

// Clear empties the internal buffer. The underlying backing array
// is removed and the memory will eventually be collected by the GC.
func (b *BufferedError) Clear() {
	b.Lock()
	b.errors = ([]error)(nil)
	b.Unlock()
}

// Add appends one or more errors to the buffer.
// Nil errors are ignored and will not be added to the buffer.
func (b *BufferedError) Add(errs ...error) {
	if len(errs) == 0 {
		return
	}

	b.Lock()
	defer b.Unlock()

	for _, e := range errs {
		if e != nil {
			if nested, ok := e.(*BufferedError); ok {
				// Prevent deadlocks and deep recursion.
				b.errors = append(b.errors, nested.Unwrap()...)
			} else {
				b.errors = append(b.errors, e)
			}
		}
	}
}

// Err returns the BufferedError itself if it contains one or more errors.
// If the buffer is empty, it returns nil. This is useful for conditional error returning
// like: `if err := buf.Err(); err != nil { return err }`.
func (b *BufferedError) Err() error {
	b.RLock()
	defer b.RUnlock()

	if len(b.errors) == 0 {
		return nil
	}

	return b
}

// New creates and returns an empty, ready-to-use BufferedError.
func New() *BufferedError {
	return &BufferedError{errors: ([]error)(nil)}
}

// NewFromError creates a new BufferedError initialized with the given error.
// If the given err is nil, it returns an empty BufferedError.
func NewFromError(err error) *BufferedError {
	if err == nil {
		return New()
	}
	return &BufferedError{errors: []error{err}}
}
