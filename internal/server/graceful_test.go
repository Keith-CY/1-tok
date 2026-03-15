package server

import (
	"testing"
	"time"
)

func TestDefaultShutdownTimeout(t *testing.T) {
	if DefaultShutdownTimeout != 30*time.Second {
		t.Errorf("default = %v", DefaultShutdownTimeout)
	}
}
