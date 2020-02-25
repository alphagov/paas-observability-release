package cursor

import (
	"time"
)

type Cursor interface {
	GetTime() time.Time
	UpdateTime(time.Time) error
}
