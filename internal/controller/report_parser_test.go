package controller

import (
	"testing"
)

func TestParseJobLogs(t *testing.T) {
	tests := []struct {
		name    string
		logs    string
		wantErr bool
		wantReq uint64
	}{
		{"valid-json", `{"latencies":{"50th":1000000},"requests":10,"success":1}`, false, 10},
		{"multiple-json-last-wins", "{\"latencies\":{\"50th\":1000},\"requests\":1,\"success\":1}\n{\"latencies\":{\"50th\":2000},\"requests\":2,\"success\":1}", false, 2},
		{"empty", "", true, 0},
		{"malformed", "{not-json}", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseJobLogs(tt.logs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && r.Requests != tt.wantReq {
				t.Fatalf("requests=%d want=%d", r.Requests, tt.wantReq)
			}
		})
	}
}

func TestReportToTargetResultDurationFormatting(t *testing.T) {
	r := &VegetaJSONReport{}
	r.Latencies.P50 = 12_300_000
	r.Latencies.P90 = 15_000_000
	r.Latencies.P95 = 16_000_000
	r.Latencies.P99 = 20_000_000
	r.Latencies.Mean = 10_500_000
	r.Latencies.Max = 30_000_000
	r.Latencies.Min = 5_000_000
	r.Requests = 100
	r.Success = 0.95
	r.Throughput = 99.9

	out := ReportToTargetResult(r, "t1", "job1")
	if out.P50 == "" || out.P99 == "" || out.Mean == "" {
		t.Fatalf("expected formatted durations, got %+v", out)
	}
	if out.SuccessRate == nil || *out.SuccessRate != 95 {
		t.Fatalf("unexpected success rate: %+v", out.SuccessRate)
	}
}
