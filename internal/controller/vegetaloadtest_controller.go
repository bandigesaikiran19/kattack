package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

const (
	cleanupFinalizer         = "loadtest.io/cleanup"
	notificationSentCondType = "NotificationSent"
)

// VegetaLoadTestReconciler reconciles a VegetaLoadTest object.
type VegetaLoadTestReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Recorder   record.EventRecorder
	KubeClient kubernetes.Interface
}

// +kubebuilder:rbac:groups=loadtest.io,resources=vegetaloadtests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadtest.io,resources=vegetaloadtests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=loadtest.io,resources=vegetaloadtests/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *VegetaLoadTestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cr := &loadtestv1alpha1.VegetaLoadTest{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(cr, cleanupFinalizer) {
			if err := r.cleanupOwnedJobs(ctx, cr); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(cr, cleanupFinalizer)
			if err := r.Update(ctx, cr); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(cr, cleanupFinalizer) {
		controllerutil.AddFinalizer(cr, cleanupFinalizer)
		if err := r.Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	now := metav1.Now()
	cr.Status.ObservedGeneration = cr.Generation

	if cr.Spec.Schedule != nil {
		if cr.Spec.Schedule.Suspend {
			cr.Status.Phase = "Suspended"
			_ = r.patchStatus(ctx, cr)
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		if !ShouldRunNow(cr.Spec.Schedule, cr.Status.LastScheduledTime, cr.Spec.Schedule.StartingDeadlineSeconds) {
			next, err := GetNextScheduleTime(cr.Spec.Schedule, cr.Status.LastScheduledTime)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !next.IsZero() {
				t := metav1.NewTime(next)
				cr.Status.NextScheduledTime = &t
				cr.Status.Phase = "Scheduled"
				_ = r.patchStatus(ctx, cr)
				return ctrl.Result{RequeueAfter: time.Until(next)}, nil
			}
		}
		if strings.EqualFold(cr.Spec.Schedule.ConcurrencyPolicy, "Forbid") && cr.Status.Phase == "Running" {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	if cr.Status.Phase == "" || cr.Status.Phase == "Pending" || cr.Status.Phase == "Scheduled" || cr.Status.Phase == "Suspended" {
		cr.Status.Phase = "Running"
		cr.Status.StartTime = &now
		cr.Status.TargetsTotal = int32(len(cr.Spec.Targets))
		cr.Status.TargetsCompleted = 0
		cr.Status.TargetsPassed = 0
		cr.Status.TargetsFailed = 0
		cr.Status.Results = []loadtestv1alpha1.TargetResult{}
	}

	for i, target := range cr.Spec.Targets {
		if i < len(cr.Status.Results) {
			phase := cr.Status.Results[i].Phase
			if phase == "Completed" || phase == "Failed" || phase == "Skipped" {
				continue
			}
		}

		cr.Status.CurrentTarget = target.Name

		jobs := &batchv1.JobList{}
		if err := r.List(ctx, jobs, client.InNamespace(cr.Namespace), client.MatchingLabels{
			"vegetaloadtest": cr.Name,
			"target":         target.Name,
		}); err != nil {
			return ctrl.Result{}, err
		}

		if len(jobs.Items) == 0 {
			if i > 0 && i-1 < len(cr.Status.Results) {
				prev := cr.Status.Results[i-1]
				if prev.Phase != "Completed" {
					prevSpec := cr.Spec.Targets[i-1]
					prevFailedThresholds := prev.ThresholdsPassed != nil && !*prev.ThresholdsPassed
					if !prevSpec.ContinueOnFailure && prevFailedThresholds {
						for j := i; j < len(cr.Spec.Targets); j++ {
							cr.Status.Results = upsertResultAt(cr.Status.Results, j, loadtestv1alpha1.TargetResult{
								TargetName: cr.Spec.Targets[j].Name,
								Phase:      "Skipped",
							})
						}
						break
					}
				}
				if wait := strings.TrimSpace(prevTargetWaitAfter(cr.Spec.Targets, i)); wait != "" {
					d, err := time.ParseDuration(wait)
					if err == nil && prev.CompletionTime != nil {
						ready := prev.CompletionTime.Time.Add(d)
						if time.Now().Before(ready) {
							_ = r.patchStatus(ctx, cr)
							return ctrl.Result{RequeueAfter: time.Until(ready)}, nil
						}
					}
				}
			}

			job := BuildVegetaJob(cr, target, i)
			if err := r.Create(ctx, job); err != nil {
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(cr, corev1.EventTypeNormal, "CreatedJob", "Created Job %s for target %s", job.Name, target.Name)
			_ = r.patchStatus(ctx, cr)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		job := newestJob(jobs.Items)
		if isJobRunning(job) {
			_ = r.patchStatus(ctx, cr)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		if isJobFinished(job) {
			logs, logErr := r.fetchJobLogs(ctx, cr.Namespace, job.Name)
			if logErr != nil {
				logger.Error(logErr, "failed to fetch job logs", "job", job.Name)
			}

			result := loadtestv1alpha1.TargetResult{TargetName: target.Name, JobName: job.Name}
			if logs != "" {
				report, err := ParseJobLogs(logs)
				if err == nil {
					result = ReportToTargetResult(report, target.Name, job.Name)
				}
			}
			if result.Phase == "" {
				result.Phase = "Completed"
			}

			mergedThresholds := mergeThresholds(cr.Spec.Thresholds, target.Thresholds)
			passed, breaches := EvaluateThresholds(&result, mergedThresholds)
			result.ThresholdsPassed = boolPtr(passed)
			result.ThresholdBreaches = breaches
			if result.StartTime == nil {
				ts := metav1.NewTime(job.CreationTimestamp.Time)
				result.StartTime = &ts
			}
			if result.CompletionTime == nil {
				ts := metav1.Now()
				result.CompletionTime = &ts
			}

			if isJobFailed(job) {
				retries := int32(0)
				retryDelay := 10 * time.Second
				if cr.Spec.ExecutionPolicy != nil {
					retries = cr.Spec.ExecutionPolicy.Retries
					if cr.Spec.ExecutionPolicy.RetryDelay != "" {
						if d, err := time.ParseDuration(cr.Spec.ExecutionPolicy.RetryDelay); err == nil {
							retryDelay = d
						}
					}
				}
				key := fmt.Sprintf("loadtest.io/retry-%s", target.Name)
				current := readRetryCount(cr, key)
				if current < retries {
					if err := r.Delete(ctx, &job); err != nil && !apierrors.IsNotFound(err) {
						return ctrl.Result{}, err
					}
					setRetryCount(cr, key, current+1)
					if err := r.Update(ctx, cr); err != nil {
						return ctrl.Result{}, err
					}
					_ = r.patchStatus(ctx, cr)
					return ctrl.Result{RequeueAfter: retryDelay}, nil
				}
				result.Phase = "Failed"
			}

			if result.Phase == "Completed" {
				cr.Status.TargetsPassed++
				r.Recorder.Eventf(cr, corev1.EventTypeNormal, "TargetCompleted", "Target %s completed", target.Name)
			} else {
				cr.Status.TargetsFailed++
				r.Recorder.Eventf(cr, corev1.EventTypeWarning, "TargetFailed", "Target %s failed", target.Name)
			}
			cr.Status.TargetsCompleted++
			cr.Status.Results = upsertResultAt(cr.Status.Results, i, result)
			RecordMetrics(cr, result)
			continue
		}
	}

	if int(cr.Status.TargetsCompleted) >= len(cr.Spec.Targets) || len(cr.Status.Results) == len(cr.Spec.Targets) {
		sum := 0.0
		count := 0.0
		allPassed := true
		anyFailed := false
		for _, rsl := range cr.Status.Results {
			if rsl.SuccessRate != nil {
				sum += *rsl.SuccessRate
				count++
			}
			if rsl.Phase == "Failed" {
				anyFailed = true
			}
			if rsl.ThresholdsPassed != nil && !*rsl.ThresholdsPassed {
				allPassed = false
			}
		}
		if count > 0 {
			avg := sum / count
			cr.Status.OverallSuccessRate = &avg
		}
		cr.Status.ThresholdsPassed = boolPtr(allPassed)
		if anyFailed {
			cr.Status.Phase = "Failed"
		} else {
			cr.Status.Phase = "Completed"
		}
		ct := metav1.Now()
		cr.Status.CompletionTime = &ct
		if cr.Spec.Schedule != nil {
			ls := metav1.Now()
			cr.Status.LastScheduledTime = &ls
			next, err := GetNextScheduleTime(cr.Spec.Schedule, cr.Status.LastScheduledTime)
			if err == nil && !next.IsZero() {
				nt := metav1.NewTime(next)
				cr.Status.NextScheduledTime = &nt
			}
		}
	}

	if cr.Spec.Notifications != nil && !hasCondition(cr.Status.Conditions, notificationSentCondType) {
		if err := SendNotification(ctx, cr, r.Client); err != nil {
			logger.Error(err, "notification errors")
		}
		cr.Status.Conditions = append(cr.Status.Conditions, metav1.Condition{
			Type:               notificationSentCondType,
			Status:             metav1.ConditionTrue,
			Reason:             "Delivered",
			Message:            "notifications attempted",
			ObservedGeneration: cr.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}

	if err := r.cleanupHistory(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.patchStatus(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VegetaLoadTestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&loadtestv1alpha1.VegetaLoadTest{}).
		Owns(&batchv1.Job{}).
		Named("vegetaloadtest").
		Complete(r)
}

func (r *VegetaLoadTestReconciler) patchStatus(ctx context.Context, cr *loadtestv1alpha1.VegetaLoadTest) error {
	payload := map[string]any{
		"apiVersion": loadtestv1alpha1.GroupVersion.String(),
		"kind":       "VegetaLoadTest",
		"metadata": map[string]any{
			"name":      cr.Name,
			"namespace": cr.Namespace,
		},
		"status": cr.Status,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.Status().Patch(ctx, cr, client.RawPatch(types.ApplyPatchType, b), client.FieldOwner("kattack"), client.ForceOwnership)
}

func (r *VegetaLoadTestReconciler) cleanupOwnedJobs(ctx context.Context, cr *loadtestv1alpha1.VegetaLoadTest) error {
	jobs := &batchv1.JobList{}
	if err := r.List(ctx, jobs, client.InNamespace(cr.Namespace), client.MatchingLabels{"vegetaloadtest": cr.Name}); err != nil {
		return err
	}
	for i := range jobs.Items {
		if err := r.Delete(ctx, &jobs.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *VegetaLoadTestReconciler) fetchJobLogs(ctx context.Context, namespace, jobName string) (string, error) {
	if r.KubeClient == nil {
		return "", nil
	}
	pods, err := r.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "job-name=" + jobName})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", nil
	}
	podName := pods.Items[0].Name
	req := r.KubeClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	b, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (r *VegetaLoadTestReconciler) cleanupHistory(ctx context.Context, cr *loadtestv1alpha1.VegetaLoadTest) error {
	successLimit := int32(3)
	failedLimit := int32(5)
	if cr.Spec.HistoryLimit != nil {
		if cr.Spec.HistoryLimit.Successful > 0 {
			successLimit = cr.Spec.HistoryLimit.Successful
		}
		if cr.Spec.HistoryLimit.Failed > 0 {
			failedLimit = cr.Spec.HistoryLimit.Failed
		}
	}

	jobs := &batchv1.JobList{}
	if err := r.List(ctx, jobs, client.InNamespace(cr.Namespace), client.MatchingLabels{"vegetaloadtest": cr.Name}); err != nil {
		return err
	}

	succeeded := make([]batchv1.Job, 0)
	failed := make([]batchv1.Job, 0)
	for _, j := range jobs.Items {
		if isJobSucceeded(j) {
			succeeded = append(succeeded, j)
		}
		if isJobFailed(j) {
			failed = append(failed, j)
		}
	}

	sort.Slice(succeeded, func(i, j int) bool { return succeeded[i].CreationTimestamp.After(succeeded[j].CreationTimestamp.Time) })
	sort.Slice(failed, func(i, j int) bool { return failed[i].CreationTimestamp.After(failed[j].CreationTimestamp.Time) })

	for i := successLimit; i < int32(len(succeeded)); i++ {
		job := succeeded[i]
		if err := r.Delete(ctx, &job); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	for i := failedLimit; i < int32(len(failed)); i++ {
		job := failed[i]
		if err := r.Delete(ctx, &job); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func newestJob(items []batchv1.Job) batchv1.Job {
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreationTimestamp.After(items[j].CreationTimestamp.Time)
	})
	return items[0]
}

func isJobRunning(job batchv1.Job) bool {
	return !isJobFinished(job)
}

func isJobFinished(job batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isJobSucceeded(job batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isJobFailed(job batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func mergeThresholds(global, target *loadtestv1alpha1.ThresholdSpec) *loadtestv1alpha1.ThresholdSpec {
	if global == nil && target == nil {
		return nil
	}
	out := &loadtestv1alpha1.ThresholdSpec{}
	if global != nil {
		*out = *global
	}
	if target != nil {
		if target.P99Latency != "" {
			out.P99Latency = target.P99Latency
		}
		if target.P95Latency != "" {
			out.P95Latency = target.P95Latency
		}
		if target.P50Latency != "" {
			out.P50Latency = target.P50Latency
		}
		if target.MeanLatency != "" {
			out.MeanLatency = target.MeanLatency
		}
		if target.MaxLatency != "" {
			out.MaxLatency = target.MaxLatency
		}
		if target.SuccessRate != nil {
			out.SuccessRate = target.SuccessRate
		}
		if target.MaxErrorRate != nil {
			out.MaxErrorRate = target.MaxErrorRate
		}
		if target.Throughput != nil {
			out.Throughput = target.Throughput
		}
	}
	return out
}

func prevTargetWaitAfter(targets []loadtestv1alpha1.TargetSpec, i int) string {
	if i <= 0 || i-1 >= len(targets) {
		return ""
	}
	return targets[i-1].WaitAfter
}

func upsertResultAt(results []loadtestv1alpha1.TargetResult, idx int, result loadtestv1alpha1.TargetResult) []loadtestv1alpha1.TargetResult {
	for len(results) <= idx {
		results = append(results, loadtestv1alpha1.TargetResult{})
	}
	results[idx] = result
	return results
}

func hasCondition(conditions []metav1.Condition, condType string) bool {
	for _, c := range conditions {
		if c.Type == condType && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func readRetryCount(cr *loadtestv1alpha1.VegetaLoadTest, key string) int32 {
	if cr.Annotations == nil {
		return 0
	}
	v := cr.Annotations[key]
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return int32(n)
}

func setRetryCount(cr *loadtestv1alpha1.VegetaLoadTest, key string, v int32) {
	if cr.Annotations == nil {
		cr.Annotations = map[string]string{}
	}
	cr.Annotations[key] = strconv.Itoa(int(v))
}

func boolPtr(v bool) *bool { return &v }
