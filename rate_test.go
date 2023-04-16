package ratelimiter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type Task struct {
	Val int
}

func (t Task) Do() {
	fmt.Printf("Time: %v, Val:%v \n", time.Now().Format(time.TimeOnly), t.Val)
	time.Sleep(time.Second * 2)
}

func TestRate_Run(t *testing.T) {
	ctx := context.Background()
	limiter := New(5, 7)

	tasks := make(chan Tasker)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		limiter.Run(ctx, tasks)
	}()

	var wg2 sync.WaitGroup

	for i := 0; i < 15; i++ {
		wg2.Add(1)
		//time.Sleep(time.Second * 3)
		go func(i int) {
			defer wg2.Done()
			tasks <- Task{i}
		}(i)
	}

	wg2.Wait()
	close(tasks)

	wg.Wait()
}
