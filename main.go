package main

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrSomeOccured        = errors.New("error some occured")
	ErrSomethingWentWrong = errors.New("something went wrong")
)

type Task struct {
	ID       int64
	CreateAt time.Time
	FinishAt time.Time
	Result   string

	err error
}

func (t *Task) Do() {
	// if t.err != nil && t.CreateAt.After(time.Now().Add(-20*time.Second)) {
	if t.err != nil && time.Since(t.CreateAt) < 20*time.Second { // - more readable than tt.After(time.Now().Add(-20 * time.Second))
		t.Result = "task has been successed"
	} else {
		t.err = ErrSomethingWentWrong
	}
	t.FinishAt = time.Now()

	time.Sleep(time.Millisecond * 150)
}

func (t Task) String() string {
	if t.err != nil {
		return fmt.Sprintf("Task id %d time %s, error: %s", t.ID, t.CreateAt, t.err.Error())
	}
	return fmt.Sprintf("Task id %d time %s, result: %s", t.ID, t.CreateAt, t.Result)
}

func NewTask() (t Task) {
	now := time.Now()

	t = Task{
		ID:       now.Unix(),
		CreateAt: now,
	}

	if t.CreateAt.Nanosecond()%2 > 0 {
		t.err = errors.New("some error occured")
	}
	return
}

func main() {
	streamTask := func(done <-chan struct{}) <-chan Task {
		ch := make(chan Task, 10)
		go func() {
			defer close(ch)
			for {
				select {
				case <-done:
					return
				default:
					ch <- NewTask()
				}
			}
		}()
		return ch
	}

	worker := func(wg *sync.WaitGroup, chResult chan<- Task, t Task) {
		defer wg.Done()
		t.Do()
		chResult <- t
	}

	doTasks := func(stream <-chan Task) <-chan Task {
		ch := make(chan Task)
		go func() {
			defer close(ch)

			var wg sync.WaitGroup
			for task := range stream {
				wg.Add(1)
				go worker(&wg, ch, task)
			}
			wg.Wait()
		}()
		return ch
	}

	sortTasks := func(stream <-chan Task) (done, undone <-chan Task) {
		chDone := make(chan Task)
		chUndone := make(chan Task)

		go func() {
			defer close(chDone)
			defer close(chUndone)

			for task := range stream {
				if task.err != nil {
					chUndone <- task
				} else {
					chDone <- task
				}
			}
		}()

		return chDone, chUndone
	}

	printer := func(buf *bytes.Buffer, stream <-chan Task) <-chan struct{} {
		done := make(chan struct{})
		go func() {
			defer close(done)

			for task := range stream {
				fmt.Fprintln(buf, task)
			}
		}()
		return done
	}

	quit := make(chan struct{})

	newTasks := streamTask(quit)
	resTasks := doTasks(newTasks)
	doneTasks, undoneTasks := sortTasks(resTasks)

	bufErr := &bytes.Buffer{}
	bufRes := &bytes.Buffer{}

	fmt.Fprintln(bufErr, "Errors:")
	fmt.Fprintln(bufRes, "Results:")

	doneErr := printer(bufErr, undoneTasks)
	doneRes := printer(bufRes, doneTasks)

	time.Sleep(3 * time.Second)
	close(quit)

	<-doneErr
	<-doneRes

	fmt.Println(bufErr.String())
	fmt.Println(bufRes.String())
}
