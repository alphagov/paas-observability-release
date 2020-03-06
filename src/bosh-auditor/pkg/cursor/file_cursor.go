package cursor

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
)

type fileCursor struct {
	name        string
	dir         string
	defaultTime time.Time

	logger lager.Logger

	mu sync.RWMutex
}

func NewFileCursor(
	name string,
	dir string,
	defaultTime time.Time,

	logger lager.Logger,
) Cursor {
	return &fileCursor{
		name:        name,
		dir:         dir,
		defaultTime: defaultTime,

		logger: logger,

		mu: sync.RWMutex{},
	}
}

func (c *fileCursor) path() (string, error) {
	joined := filepath.Join(c.dir, c.name)
	return filepath.Abs(joined)
}

func (c *fileCursor) GetTime() time.Time {
	lsession := c.logger.Session("get-time")

	c.mu.RLock()
	defer c.mu.RUnlock()

	path, err := c.path()
	if err != nil {
		lsession.Error("path", err)
		lsession.Info("path-fallback", lager.Data{"default-time": c.defaultTime})
		return c.defaultTime
	}

	contents, err := ioutil.ReadFile(path)
	if err != nil {
		lsession.Error("read-file", err)
		lsession.Info("read-file-fallback", lager.Data{"default-time": c.defaultTime})
		return c.defaultTime
	}

	tint64, err := strconv.ParseInt(string(contents), 10, 64)
	if err != nil {
		lsession.Error("parse-int", err)
		lsession.Info("parse-int-fallback", lager.Data{"default-time": c.defaultTime})
		return c.defaultTime
	}

	tunix := time.Unix(tint64, 0)
	lsession.Info("end-get-time", lager.Data{"time": tunix})
	return tunix
}

func (c *fileCursor) UpdateTime(t time.Time) error {
	lsession := c.logger.Session("update-time")

	c.mu.Lock()
	defer c.mu.Unlock()

	path, err := c.path()
	if err != nil {
		lsession.Error("path", err)
		return err
	}

	err = ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", t.Unix())), 0644)
	if err != nil {
		lsession.Error("write-file", err)
		return err
	}

	lsession.Info("end-update-time", lager.Data{"time": t})
	return nil
}
