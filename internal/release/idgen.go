package release

import (
	"fmt"
	"time"
)

var nowUTC = func() time.Time { return time.Now().UTC() }

func nanoSuffix() string {
	return fmt.Sprintf("%d", nowUTC().UnixNano())
}

func requestID() string {
	return fmt.Sprintf("req_%d", nowUTC().UnixNano())
}
