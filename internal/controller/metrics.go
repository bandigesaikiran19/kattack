package controller

import (
	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

func RecordMetrics(cr *loadtestv1alpha1.VegetaLoadTest, result loadtestv1alpha1.TargetResult) {
	_ = cr
	_ = result
}
