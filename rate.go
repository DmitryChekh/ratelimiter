package ratelimiter

import (
	"context"
	"errors"
	"time"
)

var ErrChannelClosed = errors.New("input channel closed")
var ErrInvalidLimit = errors.New("bad limit size number. It can't be less then 1")

type Tasker interface {
	Do()
}

type Rate struct {
	max          int
	maxPerMinute int
	currentCnt   int
	done         chan struct{}
	window       []time.Time // TODO: use ring buffer
}

func New(maxTasks, maxTasksPerMinute int) (*Rate, error) {
	if maxTasks < 1 || maxTasksPerMinute < 1 {
		return nil, ErrInvalidLimit
	}

	return &Rate{
		max:          maxTasks,
		maxPerMinute: maxTasksPerMinute,
		done:         make(chan struct{}),
		currentCnt:   0,
		window:       make([]time.Time, 0, maxTasksPerMinute),
	}, nil
}

func (r *Rate) Run(ctx context.Context, tasks <-chan Tasker) error {
	for {
		err := r.execTask(ctx, tasks)

		if err != nil {
			return err
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
			return ErrChannelClosed
		}

		d := r.calcWindow()

		if d != time.Duration(0) {
			select {
			case <-ctx.Done():
				err := ctx.Err()

				return err
			case <-time.After(d):
			}
		}

		if r.checkLimit() {
			select {
			case <-ctx.Done():
				err := ctx.Err()

				return err
			case <-r.done:
				r.currentCnt--
			}
		}

		r.do(task)

		return nil
	}

	return nil
}

func (r *Rate) do(t Tasker) {
	r.currentCnt++

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

	from := r.window[0]
	sub := time.Now().Sub(from)

	if sub >= time.Minute {
		return time.Duration(0)
	}

	return time.Minute - sub
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
