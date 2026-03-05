package controller

import "testing"

func TestEvaluateThresholds(t *testing.T) {
	sr := 99.0
	er := 1.0
	thr := 100.0
	tests := []struct {
		name          string
		result        TargetResult
		thresholds    ThresholdSpec
		wantPass      bool
		wantBreachesN int
	}{
		{
			name:       "p99-pass",
			result:     TargetResult{P99: "200ms", SuccessRate: &sr, ErrorRate: &er, Throughput: &thr},
			thresholds: ThresholdSpec{P99Latency: "300ms", SuccessRate: &sr},
			wantPass:   true,
		},
		{
			name:          "p99-fail",
			result:        TargetResult{P99: "823ms", SuccessRate: &sr, ErrorRate: &er, Throughput: &thr},
			thresholds:    ThresholdSpec{P99Latency: "500ms"},
			wantPass:      false,
			wantBreachesN: 1,
		},
		{
			name:          "success-rate-fail",
			result:        TargetResult{P99: "100ms", SuccessRate: ptrFloat(95), ErrorRate: &er, Throughput: &thr},
			thresholds:    ThresholdSpec{SuccessRate: ptrFloat(99)},
			wantPass:      false,
			wantBreachesN: 1,
		},
		{
			name:          "multiple-breaches",
			result:        TargetResult{P99: "900ms", Mean: "700ms", SuccessRate: ptrFloat(90), ErrorRate: ptrFloat(10), Throughput: ptrFloat(10)},
			thresholds:    ThresholdSpec{P99Latency: "100ms", MeanLatency: "100ms", SuccessRate: ptrFloat(99), MaxErrorRate: ptrFloat(1), Throughput: ptrFloat(20)},
			wantPass:      false,
			wantBreachesN: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, breaches := EvaluateThresholds(&tt.result, &tt.thresholds)
			if ok != tt.wantPass {
				t.Fatalf("pass=%v want=%v breaches=%v", ok, tt.wantPass, breaches)
			}
			if tt.wantBreachesN > 0 && len(breaches) < tt.wantBreachesN {
				t.Fatalf("breaches=%d wantAtLeast=%d", len(breaches), tt.wantBreachesN)
			}
		})
	}
}

func ptrFloat(v float64) *float64 { return &v }
