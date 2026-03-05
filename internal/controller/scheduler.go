package controller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
	"github.com/robfig/cron/v3"
)

type ScheduleSpec = loadtestv1alpha1.ScheduleSpec

func GetNextScheduleTime(schedule *ScheduleSpec, lastRun *metav1.Time) (time.Time, error) {
	if schedule == nil || schedule.Cron == "" {
		return time.Time{}, nil
	}
	loc := time.UTC
	if schedule.Timezone != "" {
		tz, err := time.LoadLocation(schedule.Timezone)
		if err != nil {
			return time.Time{}, err
		}
		loc = tz
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	s, err := parser.Parse(schedule.Cron)
	if err != nil {
		return time.Time{}, err
	}
	base := time.Now().In(loc)
	if lastRun != nil {
		base = lastRun.Time.In(loc)
	}
	return s.Next(base), nil
}

func ShouldRunNow(schedule *ScheduleSpec, lastRun *metav1.Time, deadline *int64) bool {
	next, err := GetNextScheduleTime(schedule, lastRun)
	if err != nil || next.IsZero() {
		return false
	}
	now := time.Now().In(next.Location())
	if now.Before(next) {
		return false
	}
	if deadline != nil {
		if now.Sub(next) > time.Duration(*deadline)*time.Second {
			return false
		}
	}
	return true
}
