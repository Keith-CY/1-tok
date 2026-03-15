package config

import (
	"os"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	os.Setenv("TEST_STR", "hello")
	defer os.Unsetenv("TEST_STR")
	if v := String("TEST_STR", "default"); v != "hello" {
		t.Errorf("got %s", v)
	}
	if v := String("TEST_MISSING", "default"); v != "default" {
		t.Errorf("got %s", v)
	}
}

func TestInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")
	if v := Int("TEST_INT", 0); v != 42 {
		t.Errorf("got %d", v)
	}
	if v := Int("TEST_MISSING", 10); v != 10 {
		t.Errorf("got %d", v)
	}
	os.Setenv("TEST_INT_BAD", "abc")
	defer os.Unsetenv("TEST_INT_BAD")
	if v := Int("TEST_INT_BAD", 5); v != 5 {
		t.Errorf("got %d", v)
	}
}

func TestInt64(t *testing.T) {
	os.Setenv("TEST_I64", "9999999999")
	defer os.Unsetenv("TEST_I64")
	if v := Int64("TEST_I64", 0); v != 9999999999 {
		t.Errorf("got %d", v)
	}
}

func TestBool(t *testing.T) {
	os.Setenv("TEST_BOOL", "true")
	defer os.Unsetenv("TEST_BOOL")
	if v := Bool("TEST_BOOL", false); !v {
		t.Error("expected true")
	}
	if v := Bool("TEST_MISSING", true); !v {
		t.Error("expected default true")
	}
}

func TestDuration(t *testing.T) {
	os.Setenv("TEST_DUR", "5s")
	defer os.Unsetenv("TEST_DUR")
	if v := Duration("TEST_DUR", time.Second); v != 5*time.Second {
		t.Errorf("got %v", v)
	}
}

func TestList(t *testing.T) {
	os.Setenv("TEST_LIST", "a, b, c")
	defer os.Unsetenv("TEST_LIST")
	list := List("TEST_LIST")
	if len(list) != 3 || list[0] != "a" || list[1] != "b" {
		t.Errorf("got %v", list)
	}
}

func TestList_Empty(t *testing.T) {
	list := List("TEST_EMPTY_LIST")
	if list != nil {
		t.Errorf("got %v", list)
	}
}

func TestRequiredE(t *testing.T) {
	os.Setenv("TEST_REQ", "value")
	defer os.Unsetenv("TEST_REQ")
	v, err := RequiredE("TEST_REQ")
	if err != nil || v != "value" {
		t.Errorf("got %s, %v", v, err)
	}

	_, err = RequiredE("TEST_REQ_MISSING")
	if err == nil {
		t.Error("expected error")
	}
	if _, ok := err.(*MissingEnvError); !ok {
		t.Errorf("expected MissingEnvError, got %T", err)
	}
}

func TestRequired_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	Required("DEFINITELY_MISSING_ENV_VAR")
}
