package ratelimiter

import (
	"context"
	"errors"
	"time"
)

var errChannelClosed = errors.New("input channel closed")

type Tasker interface {
	Do()
}

type Rate struct {
	max          int
	maxPerMinute int
	currentCnt   int
	done         chan struct{}
	window       []time.Time
}

func New(maxTasks, maxTasksPerMinute int) *Rate {
	return &Rate{
		max:          maxTasks,
		maxPerMinute: maxTasksPerMinute,
		done:         make(chan struct{}),
		currentCnt:   0,
		window:       make([]time.Time, 0, maxTasksPerMinute),
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
	select {
	case <-ctx.Done():
		err := ctx.Err()

		return err
	case <-r.done:
		r.currentCnt--
	case task, ok := <-tasks:
		if !ok {
			return errChannelClosed
		}

		d := r.calcWindow()

		if d != time.Duration(0) {
			select {
			case <-ctx.Done():
				return errChannelClosed
			case <-time.After(d):
			}
		}

		if r.checkLimit() {
			<-r.done
		}

		r.do(task)

		return nil
	}
	return nil
}

func (r *Rate) do(t Tasker) {
	r.currentCnt++
	r.updateWindow()

	go func() {
		t.Do()
		r.done <- struct{}{}
	}()
}

func (r *Rate) checkLimit() bool {
	return r.currentCnt == r.max
}

func (r *Rate) calcWindow() time.Duration {
	if len(r.window) < r.maxPerMinute {
		return time.Duration(0)
	}

	minuteAgo := time.Now().Truncate(time.Minute)

	from := r.window[0]
	to := r.window[len(r.window)-1]
	sub := to.Sub(from)
	if sub > time.Minute || from.Before(minuteAgo) {
		return time.Duration(0)
	}

	delay := time.Minute.Truncate(sub)

	return delay
}

func (r *Rate) updateWindow() {
	now := time.Now()

	if len(r.window) == r.maxPerMinute {
		r.window = append(r.window, now)
		r.window = r.window[1:]
	} else {
		r.window = append(r.window, now)
	}
}
