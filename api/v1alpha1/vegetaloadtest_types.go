/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretKeyRef references a specific key inside a Secret.
type SecretKeyRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key,omitempty"`
}

// ConfigMapKeyRef references a specific key inside a ConfigMap.
type ConfigMapKeyRef struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key,omitempty"`
}

// EnvVarSpec represents a key/value env var.
type EnvVarSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// VegetaLoadTestSpec defines the desired state of VegetaLoadTest.
type VegetaLoadTestSpec struct {
	// +kubebuilder:validation:MinItems=1
	Targets []TargetSpec `json:"targets"`
	Attack  AttackSpec   `json:"attack,omitempty"`
	Report  ReportSpec   `json:"report,omitempty"`

	Storage         *StorageSpec         `json:"storage,omitempty"`
	Schedule        *ScheduleSpec        `json:"schedule,omitempty"`
	Thresholds      *ThresholdSpec       `json:"thresholds,omitempty"`
	Notifications   *NotificationSpec    `json:"notifications,omitempty"`
	ExecutionPolicy *ExecutionPolicySpec `json:"executionPolicy,omitempty"`
	Resources       *ResourceSpec        `json:"resources,omitempty"`
	Image           *ImageSpec           `json:"image,omitempty"`
	HistoryLimit    *HistoryLimitSpec    `json:"historyLimit,omitempty"`

	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// TargetSpec defines one endpoint/load target.
type TargetSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`
	// +kubebuilder:default:=GET
	// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH;DELETE;HEAD;OPTIONS
	Method string `json:"method,omitempty"`

	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`

	BodySecretRef    *SecretKeyRef    `json:"bodySecretRef,omitempty"`
	BodyConfigMapRef *ConfigMapKeyRef `json:"bodyConfigMapRef,omitempty"`

	AttackOverride *AttackOverrideSpec `json:"attackOverride,omitempty"`
	Thresholds     *ThresholdSpec      `json:"thresholds,omitempty"`

	ContinueOnFailure bool   `json:"continueOnFailure,omitempty"`
	WaitAfter         string `json:"waitAfter,omitempty"`
}

// AttackSpec defines vegeta attack behavior.
type AttackSpec struct {
	// +kubebuilder:default:="50/1s"
	Rate string `json:"rate,omitempty"`
	// +kubebuilder:default:="10s"
	Duration string `json:"duration,omitempty"`
	// +kubebuilder:default:="30s"
	Timeout string `json:"timeout,omitempty"`
	// +kubebuilder:default:=10
	Workers    int32 `json:"workers,omitempty"`
	MaxWorkers int32 `json:"maxWorkers,omitempty"`
	// +kubebuilder:default:=-1
	MaxBody int64 `json:"maxBody,omitempty"`
	// +kubebuilder:default:=true
	KeepAlive bool `json:"keepAlive,omitempty"`
	// +kubebuilder:default:=true
	HTTP2    bool `json:"http2,omitempty"`
	H2C      bool `json:"h2c,omitempty"`
	Insecure bool `json:"insecure,omitempty"`
	// +kubebuilder:default:=10
	Redirects int32 `json:"redirects,omitempty"`
	Chunked   bool  `json:"chunked,omitempty"`
	// +kubebuilder:default:=10000
	Connections  int32    `json:"connections,omitempty"`
	UnixSocket   string   `json:"unixSocket,omitempty"`
	ProxyURL     string   `json:"proxyURL,omitempty"`
	LocalAddress string   `json:"localAddress,omitempty"`
	Resolvers    []string `json:"resolvers,omitempty"`
}

// AttackOverrideSpec allows per-target override of selected attack knobs.
type AttackOverrideSpec struct {
	Rate     string `json:"rate,omitempty"`
	Duration string `json:"duration,omitempty"`
	Timeout  string `json:"timeout,omitempty"`
}

// ReportSpec defines vegeta report options.
type ReportSpec struct {
	// +kubebuilder:default:=json
	// +kubebuilder:validation:Enum=text;json;hist;hdrplot
	Type    string `json:"type,omitempty"`
	Output  string `json:"output,omitempty"`
	Every   string `json:"every,omitempty"`
	Buckets string `json:"buckets,omitempty"`
}

// StorageSpec defines optional report upload storage.
type StorageSpec struct {
	// +kubebuilder:validation:Enum=s3;gcs;azure;local
	Type string `json:"type,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Bucket string `json:"bucket"`
	// +kubebuilder:default:="vegeta-reports/"
	Prefix               string        `json:"prefix,omitempty"`
	Region               string        `json:"region,omitempty"`
	Endpoint             string        `json:"endpoint,omitempty"`
	CredentialsSecretRef *SecretKeyRef `json:"credentialsSecretRef,omitempty"`
	Env                  []EnvVarSpec  `json:"env,omitempty"`
}

// ScheduleSpec defines cron-based execution.
type ScheduleSpec struct {
	Cron string `json:"cron,omitempty"`
	// +kubebuilder:default:=UTC
	Timezone                string `json:"timezone,omitempty"`
	Suspend                 bool   `json:"suspend,omitempty"`
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`
	// +kubebuilder:default:=Forbid
	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	ConcurrencyPolicy string `json:"concurrencyPolicy,omitempty"`
}

// ThresholdSpec defines pass/fail criteria.
type ThresholdSpec struct {
	P99Latency   string   `json:"p99Latency,omitempty"`
	P95Latency   string   `json:"p95Latency,omitempty"`
	P50Latency   string   `json:"p50Latency,omitempty"`
	MeanLatency  string   `json:"meanLatency,omitempty"`
	MaxLatency   string   `json:"maxLatency,omitempty"`
	SuccessRate  *float64 `json:"successRate,omitempty"`
	MaxErrorRate *float64 `json:"maxErrorRate,omitempty"`
	Throughput   *float64 `json:"throughput,omitempty"`
	// +kubebuilder:default:=true
	AbortOnFailure bool `json:"abortOnFailure,omitempty"`
}

// NotificationSpec configures notification channels.
type NotificationSpec struct {
	OnSuccess bool `json:"onSuccess,omitempty"`
	// +kubebuilder:default:=true
	OnFailure bool         `json:"onFailure,omitempty"`
	Slack     *SlackSpec   `json:"slack,omitempty"`
	Webhook   *WebhookSpec `json:"webhook,omitempty"`
	Email     *EmailSpec   `json:"email,omitempty"`
}

// SlackSpec defines Slack webhook integration.
type SlackSpec struct {
	WebhookSecretRef *SecretKeyRef `json:"webhookSecretRef,omitempty"`
	Channel          string        `json:"channel,omitempty"`
	MentionOnFailure string        `json:"mentionOnFailure,omitempty"`
}

// WebhookSpec defines a generic outbound webhook.
type WebhookSpec struct {
	URL          string            `json:"url,omitempty"`
	Method       string            `json:"method,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	TemplateBody string            `json:"templateBody,omitempty"`
}

// EmailSpec defines SMTP-based email alerts.
type EmailSpec struct {
	SmtpSecretRef *SecretKeyRef `json:"smtpSecretRef,omitempty"`
	To            []string      `json:"to,omitempty"`
	Subject       string        `json:"subject,omitempty"`
}

// ExecutionPolicySpec controls warmup and retries.
type ExecutionPolicySpec struct {
	Warmup  *WarmupSpec `json:"warmup,omitempty"`
	Retries int32       `json:"retries,omitempty"`
	// +kubebuilder:default:="10s"
	RetryDelay string `json:"retryDelay,omitempty"`
}

// WarmupSpec defines warmup attack settings.
type WarmupSpec struct {
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:default:="5/1s"
	Rate string `json:"rate,omitempty"`
	// +kubebuilder:default:="5s"
	Duration string `json:"duration,omitempty"`
}

// ResourceSpec defines pod resources/scheduling settings.
type ResourceSpec struct {
	Requests           corev1.ResourceList           `json:"requests,omitempty"`
	Limits             corev1.ResourceList           `json:"limits,omitempty"`
	NodeSelector       map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations        []corev1.Toleration           `json:"tolerations,omitempty"`
	Affinity           *corev1.Affinity              `json:"affinity,omitempty"`
	PodAnnotations     map[string]string             `json:"podAnnotations,omitempty"`
	PodLabels          map[string]string             `json:"podLabels,omitempty"`
	ImagePullSecrets   []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	ServiceAccountName string                        `json:"serviceAccountName,omitempty"`
	SecurityContext    *corev1.PodSecurityContext    `json:"securityContext,omitempty"`
}

// ImageSpec defines runner image options.
type ImageSpec struct {
	// +kubebuilder:default:=docker.io/peterevans/vegeta
	Repository string `json:"repository,omitempty"`
	// +kubebuilder:default:=latest
	Tag string `json:"tag,omitempty"`
	// +kubebuilder:default:=IfNotPresent
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// HistoryLimitSpec defines retained job history.
type HistoryLimitSpec struct {
	// +kubebuilder:default:=3
	Successful int32 `json:"successful,omitempty"`
	// +kubebuilder:default:=5
	Failed int32 `json:"failed,omitempty"`
}

// TargetResult stores one target execution result.
type TargetResult struct {
	TargetName     string       `json:"targetName,omitempty"`
	Phase          string       `json:"phase,omitempty"`
	StartTime      *metav1.Time `json:"startTime,omitempty"`
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	P50  string `json:"p50,omitempty"`
	P90  string `json:"p90,omitempty"`
	P95  string `json:"p95,omitempty"`
	P99  string `json:"p99,omitempty"`
	Mean string `json:"mean,omitempty"`
	Max  string `json:"max,omitempty"`
	Min  string `json:"min,omitempty"`

	Throughput        *float64         `json:"throughput,omitempty"`
	RequestsTotal     int64            `json:"requestsTotal,omitempty"`
	RequestsSuccess   int64            `json:"requestsSuccess,omitempty"`
	SuccessRate       *float64         `json:"successRate,omitempty"`
	ErrorRate         *float64         `json:"errorRate,omitempty"`
	StatusCodes       map[string]int64 `json:"statusCodes,omitempty"`
	ThresholdsPassed  *bool            `json:"thresholdsPassed,omitempty"`
	ThresholdBreaches []string         `json:"thresholdBreaches,omitempty"`
	ReportURL         string           `json:"reportURL,omitempty"`
	JobName           string           `json:"jobName,omitempty"`
}

// VegetaLoadTestStatus defines observed state.
type VegetaLoadTestStatus struct {
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed;Scheduled;Suspended
	Phase              string             `json:"phase,omitempty"`
	StartTime          *metav1.Time       `json:"startTime,omitempty"`
	CompletionTime     *metav1.Time       `json:"completionTime,omitempty"`
	NextScheduledTime  *metav1.Time       `json:"nextScheduledTime,omitempty"`
	LastScheduledTime  *metav1.Time       `json:"lastScheduledTime,omitempty"`
	TargetsTotal       int32              `json:"targetsTotal,omitempty"`
	TargetsCompleted   int32              `json:"targetsCompleted,omitempty"`
	TargetsPassed      int32              `json:"targetsPassed,omitempty"`
	TargetsFailed      int32              `json:"targetsFailed,omitempty"`
	CurrentTarget      string             `json:"currentTarget,omitempty"`
	ThresholdsPassed   *bool              `json:"thresholdsPassed,omitempty"`
	OverallSuccessRate *float64           `json:"overallSuccessRate,omitempty"`
	Results            []TargetResult     `json:"results,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Completed",type=integer,JSONPath=`.status.targetsCompleted`
//+kubebuilder:printcolumn:name="Passed",type=integer,JSONPath=`.status.targetsPassed`
//+kubebuilder:printcolumn:name="Failed",type=integer,JSONPath=`.status.targetsFailed`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VegetaLoadTest is the Schema for the vegetaloadtests API.
type VegetaLoadTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VegetaLoadTestSpec   `json:"spec,omitempty"`
	Status VegetaLoadTestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VegetaLoadTestList contains a list of VegetaLoadTest.
type VegetaLoadTestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VegetaLoadTest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VegetaLoadTest{}, &VegetaLoadTestList{})
}
