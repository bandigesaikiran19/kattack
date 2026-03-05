package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

func TestShouldRunNowPastAndFuture(t *testing.T) {
	now := time.Now()
	past := metav1.NewTime(now.Add(-2 * time.Minute))
	future := metav1.NewTime(now.Add(2 * time.Minute))
	s := &loadtestv1alpha1.ScheduleSpec{Cron: "* * * * *", Timezone: "UTC"}

	if !ShouldRunNow(s, &past, nil) {
		t.Fatalf("expected run now for past")
	}
	if ShouldRunNow(s, &future, nil) {
		t.Fatalf("did not expect run now for future")
	}
}

func TestShouldRunNowDeadlineEnforcement(t *testing.T) {
	last := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	s := &loadtestv1alpha1.ScheduleSpec{Cron: "* * * * *", Timezone: "UTC"}
	d := int64(30)
	if ShouldRunNow(s, &last, &d) {
		t.Fatalf("expected deadline to prevent run")
	}
}
