package jobs

import (
	"reflect"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/robfig/cron"
	"github.com/BSP-Mosaic/teltech-glog"
	"github.com/BSP-Mosaic/teltech-revel"
)

type Job struct {
	Name    string
	inner   cron.Job
	status  uint32
	running sync.Mutex
}

const UNNAMED = "(unnamed)"

func New(job cron.Job) *Job {
	name := reflect.TypeOf(job).Name()
	if name == "Func" {
		name = UNNAMED
	}
	return &Job{
		Name:  name,
		inner: job,
	}
}

func (j *Job) Status() string {
	if atomic.LoadUint32(&j.status) > 0 {
		return "RUNNING"
	}
	return "IDLE"
}

func (j *Job) Run() {
	// If the job panics, just print a stack trace.
	// Don't let the whole process die.
	defer func() {
		if err := recover(); err != nil {
			if revelError := revel.NewErrorFromPanic(err); revelError != nil {
				glog.Error(err, "\n", revelError.Stack)
			} else {
				glog.Error(err, "\n", string(debug.Stack()))
			}
		}
	}()

	if !selfConcurrent {
		j.running.Lock()
		defer j.running.Unlock()
	}

	if workPermits != nil {
		workPermits <- struct{}{}
		defer func() { <-workPermits }()
	}

	atomic.StoreUint32(&j.status, 1)
	defer atomic.StoreUint32(&j.status, 0)

	j.inner.Run()
}
