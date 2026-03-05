package controller

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

func TestBuildVegetaJobCommandFlags(t *testing.T) {
	cr := &loadtestv1alpha1.VegetaLoadTest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: loadtestv1alpha1.VegetaLoadTestSpec{
			Attack:  loadtestv1alpha1.AttackSpec{Rate: "100/1s", Duration: "20s", Timeout: "5s", Workers: 5, Redirects: 2, Connections: 100},
			Targets: []loadtestv1alpha1.TargetSpec{{Name: "a", URL: "https://example.com", Method: "GET"}},
		},
	}
	job := BuildVegetaJob(cr, cr.Spec.Targets[0], 0)
	cmd := job.Spec.Template.Spec.Containers[0].Args[0]
	for _, s := range []string{"-rate=100/1s", "-duration=20s", "-timeout=5s", "-workers=5"} {
		if !strings.Contains(cmd, s) {
			t.Fatalf("expected %q in command: %s", s, cmd)
		}
	}
}

func TestBuildVegetaJobOmitsZeroMaxWorkers(t *testing.T) {
	cr := &loadtestv1alpha1.VegetaLoadTest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: loadtestv1alpha1.VegetaLoadTestSpec{
			Attack:  loadtestv1alpha1.AttackSpec{Rate: "10/1s", Duration: "5s", Workers: 10, MaxWorkers: 0},
			Targets: []loadtestv1alpha1.TargetSpec{{Name: "a", URL: "https://example.com", Method: "GET"}},
		},
	}
	cmd := BuildVegetaJob(cr, cr.Spec.Targets[0], 0).Spec.Template.Spec.Containers[0].Args[0]
	if strings.Contains(cmd, "-max-workers=0") {
		t.Fatalf("did not expect -max-workers=0 in command: %s", cmd)
	}
}

func TestBuildVegetaJobWarmup(t *testing.T) {
	cr := &loadtestv1alpha1.VegetaLoadTest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: loadtestv1alpha1.VegetaLoadTestSpec{
			ExecutionPolicy: &loadtestv1alpha1.ExecutionPolicySpec{Warmup: &loadtestv1alpha1.WarmupSpec{Enabled: true, Rate: "5/1s", Duration: "5s"}},
			Targets:         []loadtestv1alpha1.TargetSpec{{Name: "a", URL: "https://example.com", Method: "GET"}},
		},
	}
	cmd := BuildVegetaJob(cr, cr.Spec.Targets[0], 0).Spec.Template.Spec.Containers[0].Args[0]
	if !strings.Contains(cmd, "> /dev/null") {
		t.Fatalf("expected warmup command in: %s", cmd)
	}
}

func TestBuildVegetaJobBodySecretMount(t *testing.T) {
	cr := &loadtestv1alpha1.VegetaLoadTest{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
	target := loadtestv1alpha1.TargetSpec{
		Name: "a", URL: "https://example.com", BodySecretRef: &loadtestv1alpha1.SecretKeyRef{Name: "body", Key: "payload"},
	}
	job := BuildVegetaJob(cr, target, 0)
	if len(job.Spec.Template.Spec.Volumes) == 0 {
		t.Fatalf("expected body volume mount")
	}
}

func TestBuildVegetaJobStorageRcloneAppend(t *testing.T) {
	cr := &loadtestv1alpha1.VegetaLoadTest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: loadtestv1alpha1.VegetaLoadTestSpec{
			Storage: &loadtestv1alpha1.StorageSpec{Type: "s3", Bucket: "b", Prefix: "p/"},
		},
	}
	target := loadtestv1alpha1.TargetSpec{Name: "a", URL: "https://example.com", Method: "GET"}
	cmd := BuildVegetaJob(cr, target, 0).Spec.Template.Spec.Containers[0].Args[0]
	if !strings.Contains(cmd, "rclone copy") {
		t.Fatalf("expected rclone command in: %s", cmd)
	}
}

func TestBuildVegetaJobReportOutputMirrorsToPodLogs(t *testing.T) {
	cr := &loadtestv1alpha1.VegetaLoadTest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: loadtestv1alpha1.VegetaLoadTestSpec{
			Report: loadtestv1alpha1.ReportSpec{
				Type:   "json",
				Output: "/tmp/report.json",
			},
		},
	}
	target := loadtestv1alpha1.TargetSpec{Name: "a", URL: "https://example.com", Method: "GET"}
	cmd := BuildVegetaJob(cr, target, 0).Spec.Template.Spec.Containers[0].Args[0]
	if !strings.Contains(cmd, "| tee /tmp/report.json") {
		t.Fatalf("expected report output to be tee'd for logs and file: %s", cmd)
	}
	if strings.Contains(cmd, "-output=/tmp/report.json") {
		t.Fatalf("did not expect vegeta report -output flag because it suppresses stdout logs: %s", cmd)
	}
}
