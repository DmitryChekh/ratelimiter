package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errChannelClosed = errors.New("input channel closed")

type Tasker interface {
	Do()
}

type Rate struct {
	max          int
	maxPerMinute int

	waitq []Tasker

	mutex         sync.Mutex
	currentCnt    int
	lastMinuteCnt int

	done           chan struct{}
	lastMinuteDone chan struct{}
}

func New(maxTasks, maxTasksPerMinute int) *Rate {
	return &Rate{
		max:            maxTasks,
		maxPerMinute:   maxTasksPerMinute,
		mutex:          sync.Mutex{},
		done:           make(chan struct{}),
		lastMinuteDone: make(chan struct{}),
		currentCnt:     0,
		lastMinuteCnt:  0,
		waitq:          make([]Tasker, 0),
	}
}

func (r *Rate) Run(ctx context.Context, tasks <-chan Tasker) {
	for {
		err := r.execTask(ctx, tasks)
		if err != nil {
			return
		}
	}
}

func (r *Rate) execTask(ctx context.Context, tasks <-chan Tasker) error {
	if len(r.waitq) > 0 {
		if ok := r.tryRunFromQueue(); !ok {
			select {
			case <-ctx.Done():
				err := ctx.Err()

				return err
			case <-r.done:
				r.currentCnt--
			case <-r.lastMinuteDone:
				r.lastMinuteCnt--
			}
		}

		return nil
	}

	select {
	case <-ctx.Done():
		err := ctx.Err()

		return err
	case task, ok := <-tasks:
		if !ok {
			return errChannelClosed
		}

		if r.checkLimit() {
			r.waitq = append(r.waitq, task)

			return nil
		}

		r.do(task)

		return nil
	}
}

func (r *Rate) tryRunFromQueue() bool {
	if r.checkLimit() {
		return false
	}

	first := r.waitq[0]
	r.waitq = r.waitq[1:]

	r.do(first)

	return true
}

func (r *Rate) do(t Tasker) {
	r.currentCnt++
	r.lastMinuteCnt++

	go func() {
		t.Do()
		r.execDone()
	}()
}

func (r *Rate) execDone() {
	r.done <- struct{}{}
	time.Sleep(time.Minute)
	r.lastMinuteDone <- struct{}{}
}

func (r *Rate) checkLimit() bool {
	return r.currentCnt == r.max || r.lastMinuteCnt == r.maxPerMinute
}
