package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

type TargetResult = loadtestv1alpha1.TargetResult

type VegetaJSONReport struct {
	Latencies struct {
		P50  uint64 `json:"50th"`
		P90  uint64 `json:"90th"`
		P95  uint64 `json:"95th"`
		P99  uint64 `json:"99th"`
		Mean uint64 `json:"mean"`
		Max  uint64 `json:"max"`
		Min  uint64 `json:"min"`
	} `json:"latencies"`
	BytesIn struct {
		Total uint64 `json:"total"`
		Mean  uint64 `json:"mean"`
	} `json:"bytes_in"`
	BytesOut struct {
		Total uint64 `json:"total"`
		Mean  uint64 `json:"mean"`
	} `json:"bytes_out"`
	Earliest    time.Time      `json:"earliest"`
	Latest      time.Time      `json:"latest"`
	End         time.Time      `json:"end"`
	Duration    uint64         `json:"duration"`
	Wait        uint64         `json:"wait"`
	Requests    uint64         `json:"requests"`
	Rate        float64        `json:"rate"`
	Throughput  float64        `json:"throughput"`
	Success     float64        `json:"success"`
	StatusCodes map[string]int `json:"status_codes"`
	Errors      []string       `json:"errors"`
}

func ParseJobLogs(logs string) (*VegetaJSONReport, error) {
	if strings.TrimSpace(logs) == "" {
		return nil, errors.New("no valid JSON found in logs")
	}
	lines := strings.Split(logs, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var report VegetaJSONReport
		if err := json.Unmarshal([]byte(line), &report); err == nil {
			return &report, nil
		}
	}
	return nil, errors.New("no valid JSON found in logs")
}

func ReportToTargetResult(report *VegetaJSONReport, targetName, jobName string) TargetResult {
	now := metav1.Now()
	successRate := report.Success * 100
	errorRate := (1 - report.Success) * 100
	requestsSuccess := int64(float64(report.Requests) * report.Success)

	statusCodes := make(map[string]int64, len(report.StatusCodes))
	for k, v := range report.StatusCodes {
		statusCodes[k] = int64(v)
	}

	return TargetResult{
		TargetName:      targetName,
		Phase:           "Completed",
		CompletionTime:  &now,
		P50:             nsToDuration(report.Latencies.P50),
		P90:             nsToDuration(report.Latencies.P90),
		P95:             nsToDuration(report.Latencies.P95),
		P99:             nsToDuration(report.Latencies.P99),
		Mean:            nsToDuration(report.Latencies.Mean),
		Max:             nsToDuration(report.Latencies.Max),
		Min:             nsToDuration(report.Latencies.Min),
		Throughput:      float64Ptr(report.Throughput),
		RequestsTotal:   int64(report.Requests),
		RequestsSuccess: requestsSuccess,
		SuccessRate:     float64Ptr(successRate),
		ErrorRate:       float64Ptr(errorRate),
		StatusCodes:     statusCodes,
		JobName:         jobName,
	}
}

func nsToDuration(v uint64) string {
	return fmt.Sprintf("%s", time.Duration(v).Round(100*time.Microsecond))
}

func float64Ptr(v float64) *float64 { return &v }
