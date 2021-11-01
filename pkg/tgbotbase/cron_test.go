package tgbotbase

import "testing"
import "time"
import "math/rand"
import "sync/atomic"

type testCronCountingJob struct {
	count          int32
	repeat         *time.Duration
	repeatMaxCount int32
}

func (j *testCronCountingJob) Do(t time.Time, c Cron) {
	atomic.AddInt32(&j.count, 1) // atomic to avoid race detector
	if (j.repeat != nil) && j.count < j.repeatMaxCount {
		c.AddJob(t.Add(*j.repeat), j)
	}
}

func TestCallOnce(t *testing.T) {
	c := NewCron()
	j := &testCronCountingJob{}
	c.AddJob(time.Now(), j)
	time.Sleep(100 * time.Millisecond)
	atomic.LoadInt32(&j.count)
	if j.count != 1 {
		t.Fatal(j.count)
	}
}

func TestCallXTimes(t *testing.T) {
	c := NewCron()
	j := &testCronCountingJob{}

	now := time.Now()
	n := 5 + rand.Int31n(5)
	var i int32
	for ; i < n; i++ {
		c.AddJob(now, j)
	}

	time.Sleep(100 * time.Millisecond)
	atomic.LoadInt32(&j.count)
	if j.count != n {
		t.Fatal(j.count, n)
	}
}

func TestDifferentTimesRandom(t *testing.T) {
	durations := []int{1, 2, 3, 4, 5, 6, 7}
	rand.Shuffle(len(durations), func(i int, j int) {
		durations[i], durations[j] = durations[j], durations[i]
	})

	c := NewCron()
	j := &testCronCountingJob{}
	now := time.Now()
	for i := 0; i < len(durations); i++ {
		c.AddJob(now.Add(time.Duration(durations[i])*100*time.Millisecond), j)
	}
	time.Sleep(time.Duration(len(durations)+1) * 100 * time.Millisecond)
	atomic.LoadInt32(&j.count)
	if j.count != int32(len(durations)) {
		t.Fatal(j.count, len(durations))
	}
}

func TestDifferentTimesAsc(t *testing.T) {
	durations := []int{1, 2, 3, 4, 5, 6, 7}

	c := NewCron()
	j := &testCronCountingJob{}
	now := time.Now()
	for i := 0; i < len(durations); i++ {
		c.AddJob(now.Add(time.Duration(durations[i])*100*time.Millisecond), j)
	}
	time.Sleep(time.Duration(len(durations)+1) * 100 * time.Millisecond)
	atomic.LoadInt32(&j.count)
	if j.count != int32(len(durations)) {
		t.Fatal(j.count, len(durations))
	}
}

func TestDifferentTimesDesc(t *testing.T) {
	durations := []int{7, 6, 5, 4, 3, 2, 1}

	c := NewCron()
	j := &testCronCountingJob{}
	now := time.Now()
	for i := 0; i < len(durations); i++ {
		c.AddJob(now.Add(time.Duration(durations[i])*100*time.Millisecond), j)
	}
	time.Sleep(time.Duration(len(durations)+1) * 100 * time.Millisecond)
	atomic.LoadInt32(&j.count)
	if j.count != int32(len(durations)) {
		t.Fatal(j.count, len(durations))
	}
}

func TestRepeatXTimes(t *testing.T) {
	c := NewCron()
	repeat := 100 * time.Millisecond
	repeatN := 3 + rand.Int31n(3)
	j := &testCronCountingJob{
		repeat:         &repeat,
		repeatMaxCount: repeatN}

	c.AddJob(time.Now(), j)

	time.Sleep(time.Second)
	atomic.LoadInt32(&j.count)
	if j.count != repeatN {
		t.Fatal(j.count, repeatN)
	}
}
