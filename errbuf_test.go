package errbuf

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewFromError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		buf := NewFromError(nil)
		if len(buf.errors) != 0 {
			t.Errorf("expected 0 errors, got %d", len(buf.errors))
		}
	})

	t.Run("valid error", func(t *testing.T) {
		err := errors.New("test")
		buf := NewFromError(err)
		if len(buf.errors) != 1 || buf.errors[0] != err {
			t.Errorf("expected [test], got %v", buf.errors)
		}
	})
}

func TestBufferedError_Add(t *testing.T) {
	buf := New()
	buf.Add(nil)
	if len(buf.errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(buf.errors))
	}

	err1 := errors.New("err1")
	err2 := errors.New("err2")

	buf.Add(err1)
	if len(buf.errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(buf.errors))
	}

	buf.Add(err2, nil, err1)
	if len(buf.errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(buf.errors))
	}
}

func TestBufferedError_Err(t *testing.T) {
	buf := New()
	if buf.Err() != nil {
		t.Errorf("expected nil, got %v", buf.Err())
	}

	buf.Add(errors.New("test"))
	if buf.Err() == nil {
		t.Errorf("expected non-nil, got %v", buf.Err())
	}
}

func TestBufferedError_Error(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		buf := New()
		if got := buf.Error(); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("single", func(t *testing.T) {
		buf := NewFromError(errors.New("err1"))
		if got := buf.Error(); got != "err1" {
			t.Errorf("expected 'err1', got %q", got)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		buf := New()
		buf.Add(errors.New("err1"), errors.New("err2"), errors.New("err3"))
		expected := "err1; err2; err3"
		if got := buf.Error(); got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestBufferedError_Format(t *testing.T) {
	buf := New()
	buf.Add(errors.New("err1"), errors.New("err2"))

	t.Run("default string (%s)", func(t *testing.T) {
		got := fmt.Sprintf("%s", buf)
		expected := "err1; err2"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("v verb (%v) multiline", func(t *testing.T) {
		got := fmt.Sprintf("%v", buf)
		expected := "err1\nerr2"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("plus v verb (%+v)", func(t *testing.T) {
		got := fmt.Sprintf("%+v", buf)
		expected := "BufferedError{errors:[err1 err2]}"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("nested multiline (%v)", func(t *testing.T) {
		parent := New()
		nested := New()
		nested.Add(errors.New("nested1"), errors.New("nested2"))
		parent.Add(errors.New("parent1"), nested, errors.New("parent2"))

		got := fmt.Sprintf("%v", parent)
		expected := "parent1\nnested1\nnested2\nparent2"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestBufferedError_Unwrap(t *testing.T) {
	buf := New()
	err1, err2 := errors.New("1"), errors.New("2")
	buf.Add(err1, err2)

	unwrapped := buf.Unwrap()
	if len(unwrapped) != 2 || unwrapped[0] != err1 || unwrapped[1] != err2 {
		t.Errorf("unexpected unwrap result: %v", unwrapped)
	}

	// modify unwrapped slice to ensure it's a copy
	unwrapped[0] = nil
	if len(buf.errors) == 0 || buf.errors[0] == nil {
		t.Errorf("Unwrap did not return a copy")
	}
}

func TestBufferedError_Clear(t *testing.T) {
	buf := New()
	buf.Add(errors.New("1"))
	buf.Clear()
	if len(buf.errors) != 0 {
		t.Errorf("expected 0 errors after Clear(), got %d", len(buf.errors))
	}
}

func TestBufferedError_IsAs(t *testing.T) {
	var errTarget = errors.New("target error")
	buf := New()
	buf.Add(errors.New("other 1"), errTarget, errors.New("other 2"))

	if !errors.Is(buf, errTarget) {
		t.Errorf("expected errors.Is to find the target error in the buffer")
	}

	if errors.Is(buf, errors.New("not present")) {
		t.Errorf("expected errors.Is to return false for non-present error")
	}
}

// targetErrType is a custom error type used to test errors.As
type targetErrType struct {
	msg string
}

func (e *targetErrType) Error() string { return e.msg }

func TestBufferedError_As(t *testing.T) {
	errTarget := &targetErrType{msg: "target"}
	buf := New()
	buf.Add(errors.New("other 1"), errTarget, errors.New("other 2"))

	var matched *targetErrType
	if !errors.As(buf, &matched) {
		t.Errorf("expected errors.As to find the target error type in the buffer")
	}
	if matched == nil || matched.msg != "target" {
		t.Errorf("expected the matched error to be populated")
	}
}

func TestBufferedError_AddFlattening(t *testing.T) {
	nested := New()
	nested.Add(errors.New("nested 1"), errors.New("nested 2"))
	
	parent := New()
	parent.Add(errors.New("parent 1"), nested, errors.New("parent 2"))

	unwrapped := parent.Unwrap()
	if len(unwrapped) != 4 {
		t.Errorf("expected parent to have 4 errors after flattening nested buffer, got %d", len(unwrapped))
	}
	
	if unwrapped[1].Error() != "nested 1" || unwrapped[2].Error() != "nested 2" {
		t.Errorf("expected nested errors to be correctly unrolled in parent buffer")
	}
}

func TestBufferedError_Grow(t *testing.T) {
	buf := New()
	buf.Grow(10)
	
	// Ensure it copies existing elements using internal buffer tests
	buf.Add(errors.New("1"))
	buf.Grow(20)
	buf.RLock()
	if len(buf.errors) != 1 {
		t.Errorf("expected length 1, got %d", len(buf.errors))
	}
	buf.RUnlock()
}

var errs = make([]error, 1001)

func init() {
	for i := range errs {
		if i == 0 {
			errs[i] = nil
		} else {
			errs[i] = errors.New("test error")
		}
	}
}

func BenchmarkBufferedErrorAddSlice1000(b *testing.B) {
	buf := New()
	buf.Grow(len(errs))
	b.ResetTimer()
	for range b.N {
		for _, err := range errs {
			buf.Add(err)
		}
		_ = buf.Error()
		buf.Clear()
	}
}

func BenchmarkBufferedErrorAddSlice1000All(b *testing.B) {
	buf := New()
	b.ResetTimer()
	for range b.N {
		buf.Add(errs...)
		_ = buf.Error()
		buf.Clear()
	}
}

func BenchmarkBufferedErrorAdd1(b *testing.B) {
	err1 := errors.New("test error")
	buf := New()
	b.ResetTimer()
	for range b.N {
		buf.Add(err1)
		_ = buf.Error()
		buf.Clear()
	}
}

func BenchmarkBufferedErrorAddParallel(b *testing.B) {
	buf := New()
	err1 := errors.New("test error")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf.Add(err1)
		}
	})
	
	// Error is measured outside since Parallel tests the thread safety of Adds
	_ = buf.Error()
}

func BenchmarkBuffereErrorMultiLine(b *testing.B) {
	buf := NewFromError(nil)
	buf.Grow(len(errs))
	b.ResetTimer()
	for range b.N {
		for _, err := range errs {
			buf.Add(err)
		}
		_ = fmt.Sprintf("%v", buf)
		buf.Clear()
	}
}
