package release

import (
	"strconv"
	"time"
)

var nowUTC = func() time.Time { return time.Now().UTC() }

func nanoSuffix() string {
	return strconv.FormatInt(nowUTC().UnixNano(), 10)
}

func requestID() string {
	return "req_" + nanoSuffix()
}
