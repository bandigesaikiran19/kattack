package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

type ThresholdSpec = loadtestv1alpha1.ThresholdSpec

func EvaluateThresholds(result *TargetResult, thresholds *ThresholdSpec) (bool, []string) {
	if result == nil || thresholds == nil {
		return true, nil
	}

	breaches := make([]string, 0)
	compareLatency := func(label, val, threshold string) {
		if threshold == "" || val == "" {
			return
		}
		vDur, err := time.ParseDuration(val)
		if err != nil {
			breaches = append(breaches, fmt.Sprintf("%s %s could not be parsed", label, val))
			return
		}
		tDur, err := time.ParseDuration(threshold)
		if err != nil {
			breaches = append(breaches, fmt.Sprintf("%s threshold %s is invalid", label, threshold))
			return
		}
		if vDur > tDur {
			breaches = append(breaches, fmt.Sprintf("%s latency %s exceeded threshold %s", label, vDur, tDur))
		}
	}

	compareLatency("p99", result.P99, thresholds.P99Latency)
	compareLatency("p95", result.P95, thresholds.P95Latency)
	compareLatency("p50", result.P50, thresholds.P50Latency)
	compareLatency("mean", result.Mean, thresholds.MeanLatency)
	compareLatency("max", result.Max, thresholds.MaxLatency)

	if thresholds.SuccessRate != nil && result.SuccessRate != nil && *result.SuccessRate < *thresholds.SuccessRate {
		breaches = append(breaches, fmt.Sprintf("success rate %.2f%% below threshold %.2f%%", *result.SuccessRate, *thresholds.SuccessRate))
	}
	if thresholds.MaxErrorRate != nil && result.ErrorRate != nil && *result.ErrorRate > *thresholds.MaxErrorRate {
		breaches = append(breaches, fmt.Sprintf("error rate %.2f%% exceeded threshold %.2f%%", *result.ErrorRate, *thresholds.MaxErrorRate))
	}
	if thresholds.Throughput != nil && result.Throughput != nil && *result.Throughput < *thresholds.Throughput {
		breaches = append(breaches, fmt.Sprintf("throughput %.2f below threshold %.2f", *result.Throughput, *thresholds.Throughput))
	}

	return len(breaches) == 0, breaches
}

func parsePercentString(v string) (float64, error) {
	v = strings.TrimSpace(strings.TrimSuffix(v, "%"))
	return strconv.ParseFloat(v, 64)
}
