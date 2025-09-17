// Package errbuf contains errors buffer, which allows to collect errors
// before processing them. In addition this buffer can hold warnings, which are
// the same error type, but can be accessed via their own methods and processed
// accordingly. This buffer is goroutine safe.
package errbuf

import "sync"

// BufferedError represents cache to collect errors during the process before fail.
// Also allows to collect not critical warnings which can be just reported instead.
type BufferedError struct {
	sync.RWMutex

	errors []error

	// Warnings is another buffer to collect not critical errors.
	// It can collect errors, which does not affect the outcome
	// in general, but should be logged or processed as well.
	// This slice allows to collect all the warnings. It can be
	// checked with [ShouldWarn] method and has additional
	// stringer [Warning] method.
	warnings []error
}

// Error formats errors in the buffer into the string.
func (buf *BufferedError) Error() string {
	buf.Lock()
	defer buf.Unlock()

	return bufToString(buf.errors)
}

// Warning formats warnings in the buffer into the string.
func (buf *BufferedError) Warning() string {
	buf.Lock()
	defer buf.Unlock()

	return bufToString(buf.warnings)
}

// bufToString converts given slice of errors into the string.
func bufToString(sl []error) string {
	var b []byte

	for i, err := range sl {
		if i > 0 {
			b = append(b, '\n')
		}

		b = append(b, err.Error()...)
	}

	return string(b)
}

// Unwrap returns copy of the buffer errors.
func (buf *BufferedError) Unwrap() []error {
	buf.Lock()
	defer buf.Unlock()

	return append([]error{}, buf.errors...)
}

// Add adds given error to the errors buffer.
func (buf *BufferedError) Add(err error) {
	buf.Lock()
	defer buf.Unlock()

	buf.errors = append(buf.errors, err)
}

// Warn adds given error to the warnings buffer.
func (buf *BufferedError) Warn(err error) {
	buf.Lock()
	defer buf.Unlock()

	buf.warnings = append(buf.warnings, err)
}

// Err returns nil if error buffer is empty.
func (buf *BufferedError) Err() error {
	buf.Lock()
	defer buf.Unlock()

	if len(buf.errors) == 0 {
		return nil
	}

	return buf
}

// ShouldWarn returns true if warnings buffer is not empty.
func (buf *BufferedError) ShouldWarn() bool {
	buf.Lock()
	defer buf.Unlock()

	return len(buf.warnings) != 0
}

// NewErrorsBuffer creates empty [BufferedError].
func NewErrorsBuffer() *BufferedError {
	return &BufferedError{errors: make([]error, 0), warnings: make([]error, 0)}
}

// NewBufferFromError creates new [BufferedError] from the given error.
func NewBufferFromError(err error) *BufferedError {
	return &BufferedError{errors: []error{err}, warnings: make([]error, 0)}
}

// NewBufferFromWarning creates new [BuffereError] and adds given error to warnings.
func NewBufferFromWarning(err error) *BufferedError {
	return &BufferedError{errors: make([]error, 0), warnings: []error{err}}
}
