package tgbotbase

import "time"
import "log"
import "sort"
import "math"

// Cron interface declares interfaces for communication with some cron daemon
type Cron interface {
	AddJob(when time.Time, job CronJob)
}

// CronJob provides a piece of work which should be done once its time has come
type CronJob interface {
	Do(scheduledWhen time.Time, cron Cron)
}

type cronJobDesc struct {
	execTime time.Time
	job      CronJob
}

type cron struct {
	newJobCh chan cronJobDesc
	timer    *time.Timer

	jobs           map[time.Time][]CronJob
	sortedJobTimes []time.Time
}

var maxTimerDuration time.Duration = time.Duration(math.MaxInt64) * time.Nanosecond

func (c *cron) AddJob(t time.Time, job CronJob) {
	c.newJobCh <- cronJobDesc{
		execTime: t,
		job:      job}
}

func (c *cron) executeJobs(jobsToExecute map[time.Time][]CronJob, now time.Time) {
	for scheduledTime, jobs := range jobsToExecute {
		log.Printf("cron: Executing %d jobs at time %s (scheduled %s; diff %s)", len(jobs), now, scheduledTime, now.Sub(scheduledTime))
		for _, j := range jobs {
			go j.Do(scheduledTime, c)
		}
	}
}

func (c *cron) processNewJob(execTime time.Time, job CronJob) {
	if _, found := c.jobs[execTime]; found {
		log.Printf("cron: New job with known time %s has arrived", execTime)
		c.jobs[execTime] = append(c.jobs[execTime], job)
	} else {
		log.Printf("cron: New job with not yet known time %s has arrived", execTime)
		c.jobs[execTime] = []CronJob{job}
		c.sortedJobTimes = append(c.sortedJobTimes, execTime)
		sort.Slice(c.sortedJobTimes, func(i int, j int) bool {
			return c.sortedJobTimes[i].Before(c.sortedJobTimes[j])
		})
		c.resetTimer(time.Now())
	}
}

func (c *cron) resetTimer(now time.Time) {
	log.Printf("cron: timer is going to be reset")
	nextTimer := maxTimerDuration
	if len(c.sortedJobTimes) > 0 {
		nextTimer = c.sortedJobTimes[0].Sub(now)
	}

	log.Printf("cron: Timer will be reset to %s (now %s + duration %s)", now.Add(nextTimer), now, nextTimer)
	if !c.timer.Stop() {
		select {
		case <-c.timer.C:
		default:
		}
	}
	c.timer.Reset(nextTimer)
}

func (c *cron) run() {
	isRunning := true
	for isRunning {
		select {
		case j := <-c.newJobCh:
			log.Printf("cron: Received new job for time %s", j.execTime)
			c.processNewJob(j.execTime, j.job)
		case now := <-c.timer.C:
			log.Printf("cron: New trigger tick: %s; registered times: %d", now, len(c.sortedJobTimes))
			pos := sort.Search(len(c.sortedJobTimes), func(i int) bool {
				return now.Before(c.sortedJobTimes[i])
			})
			if pos == len(c.sortedJobTimes) {
				panic("cron: scheduling inconsistency")
			}
			// preparing list of jobs which should be executed, removing them from internal structures
			jobsToExecute := make(map[time.Time][]CronJob, pos+1)
			for i := 0; i < pos; i++ {
				t := c.sortedJobTimes[i]
				jobsToExecute[t] = c.jobs[t]
				delete(c.jobs, t)
			}
			c.sortedJobTimes = c.sortedJobTimes[pos:]
			log.Printf("cron: after preparing jobs for execution: %d times left", len(c.sortedJobTimes))
			if len(jobsToExecute) == 0 {
				panic("cron: time-to-jobs inconsistency")
			}
			if len(c.jobs) != len(c.sortedJobTimes)-1 { // correction for 'fake' bit value
				panic("cron: job map and sorted times list size mismatch")
			}
			c.executeJobs(jobsToExecute, now)
			c.resetTimer(now)
		}
	}
}

// NewCron creates an instance of cron
func NewCron() Cron {
	now := time.Now()
	c := cron{
		newJobCh:       make(chan cronJobDesc, 0),
		jobs:           make(map[time.Time][]CronJob, 0),
		sortedJobTimes: []time.Time{now.Add(maxTimerDuration)}, // setting bit value for sort.Search to work correctly
		timer:          time.NewTimer(maxTimerDuration)}

	go c.run()
	log.Printf("New cron has started")

	return &c
}

func CalcNextTimeFromMidnight(now time.Time, fromMidnight time.Duration) time.Time {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	nextTime := midnight.Add(fromMidnight)
	if nextTime.Before(now) {
		nextTime = nextTime.Add(24 * time.Hour)
	}
	return nextTime
}
