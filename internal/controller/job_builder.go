package controller

import (
	"fmt"
	"path"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

type TargetSpec = loadtestv1alpha1.TargetSpec

type attackConfig struct {
	Rate         string
	Duration     string
	Timeout      string
	Workers      int32
	MaxWorkers   int32
	MaxBody      int64
	KeepAlive    bool
	HTTP2        bool
	H2C          bool
	Insecure     bool
	Redirects    int32
	Chunked      bool
	Connections  int32
	UnixSocket   string
	ProxyURL     string
	LocalAddress string
	Resolvers    []string
}

func BuildVegetaJob(cr *loadtestv1alpha1.VegetaLoadTest, target TargetSpec, index int) *batchv1.Job {
	labels := map[string]string{
		"app":            "kattack",
		"vegetaloadtest": cr.Name,
		"target":         target.Name,
	}

	jobName := fmt.Sprintf("%s-%s-%d", cr.Name, target.Name, index)

	attack := buildAttackConfig(cr.Spec.Attack, target.AttackOverride)
	reportType := cr.Spec.Report.Type
	if reportType == "" {
		reportType = "json"
	}

	container := corev1.Container{
		Name:            "vegeta",
		Image:           resolveImage(cr.Spec.Image),
		ImagePullPolicy: resolvePullPolicy(cr.Spec.Image),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{buildJobCommand(cr, target, attack, reportType)},
	}

	if cr.Spec.Resources != nil {
		container.Resources = corev1.ResourceRequirements{
			Requests: cr.Spec.Resources.Requests,
			Limits:   cr.Spec.Resources.Limits,
		}
	}

	if target.BodySecretRef != nil || target.BodyConfigMapRef != nil {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "target-body",
			MountPath: "/etc/vegeta/body",
			ReadOnly:  true,
		})
	}

	if target.BodySecretRef != nil {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: "TARGET_BODY_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: target.BodySecretRef.Name},
					Key:                  target.BodySecretRef.Key,
				},
			},
		})
	}

	if cr.Spec.Storage != nil {
		for _, e := range cr.Spec.Storage.Env {
			container.Env = append(container.Env, corev1.EnvVar{Name: e.Name, Value: e.Value})
		}
		if cr.Spec.Storage.CredentialsSecretRef != nil {
			container.Env = append(container.Env, corev1.EnvVar{
				Name: "STORAGE_CREDENTIAL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: cr.Spec.Storage.CredentialsSecretRef.Name},
						Key:                  cr.Spec.Storage.CredentialsSecretRef.Key,
					},
				},
			})
		}
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers:    []corev1.Container{container},
	}

	if target.BodySecretRef != nil {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "target-body",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
				SecretName: target.BodySecretRef.Name,
				Items: []corev1.KeyToPath{{
					Key:  target.BodySecretRef.Key,
					Path: "body",
				}},
			}},
		})
	}
	if target.BodyConfigMapRef != nil {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "target-body",
			VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: target.BodyConfigMapRef.Name},
				Items: []corev1.KeyToPath{{
					Key:  target.BodyConfigMapRef.Key,
					Path: "body",
				}},
			}},
		})
	}

	if cr.Spec.Resources != nil {
		podSpec.NodeSelector = cr.Spec.Resources.NodeSelector
		podSpec.Tolerations = cr.Spec.Resources.Tolerations
		podSpec.Affinity = cr.Spec.Resources.Affinity
		podSpec.ImagePullSecrets = cr.Spec.Resources.ImagePullSecrets
		podSpec.ServiceAccountName = cr.Spec.Resources.ServiceAccountName
		podSpec.SecurityContext = cr.Spec.Resources.SecurityContext
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
			BackoffLimit: int32Ptr(0),
		},
	}

	if cr.Spec.Resources != nil {
		if len(cr.Spec.Resources.PodAnnotations) > 0 {
			job.Spec.Template.Annotations = cr.Spec.Resources.PodAnnotations
		}
		if len(cr.Spec.Resources.PodLabels) > 0 {
			for k, v := range cr.Spec.Resources.PodLabels {
				job.Spec.Template.Labels[k] = v
			}
		}
	}

	if cr.Spec.TTLSecondsAfterFinished != nil {
		job.Spec.TTLSecondsAfterFinished = cr.Spec.TTLSecondsAfterFinished
	}

	_ = controllerutil.SetControllerReference(cr, job, buildScheme(cr))
	return job
}

func buildJobCommand(cr *loadtestv1alpha1.VegetaLoadTest, target TargetSpec, attack attackConfig, reportType string) string {
	targetLine := fmt.Sprintf("%s %s", methodOrDefault(target.Method), target.URL)
	if target.Body != "" {
		targetLine += "\\n" + target.Body
	} else if target.BodySecretRef != nil || target.BodyConfigMapRef != nil {
		targetLine += "\\n@/etc/vegeta/body/body"
	}
	targetLineEscaped := shellEscape(targetLine)

	reportCmd := fmt.Sprintf("vegeta report -type=%s report.bin", reportType)
	if cr.Spec.Report.Every != "" {
		reportCmd += " -every=" + shellEscape(cr.Spec.Report.Every)
	}
	if cr.Spec.Report.Buckets != "" {
		reportCmd += " -buckets=" + shellEscape(cr.Spec.Report.Buckets)
	}

	mainAttack := fmt.Sprintf(
		"echo '[vegeta] target=%s url=%s'; printf %%b\\\\n %s | vegeta attack %s > report.bin; echo '[vegeta] report'; %s",
		target.Name,
		target.URL,
		targetLineEscaped,
		buildAttackFlags(attack),
		reportCmd,
	)
	if cr.Spec.Report.Output != "" {
		// Mirror report output to file while keeping stdout for pod logs/status parsing.
		mainAttack += " | tee " + shellEscape(cr.Spec.Report.Output)
	}
	if cr.Spec.Storage != nil {
		prefix := cr.Spec.Storage.Prefix
		if prefix == "" {
			prefix = "vegeta-reports/"
		}
		mainAttack += fmt.Sprintf("; rclone copy report.bin %s:%s", cr.Spec.Storage.Bucket, path.Join(prefix, cr.Name, target.Name+".bin"))
	}

	if cr.Spec.ExecutionPolicy != nil && cr.Spec.ExecutionPolicy.Warmup != nil && cr.Spec.ExecutionPolicy.Warmup.Enabled {
		warmRate := cr.Spec.ExecutionPolicy.Warmup.Rate
		if warmRate == "" {
			warmRate = "5/1s"
		}
		warmDur := cr.Spec.ExecutionPolicy.Warmup.Duration
		if warmDur == "" {
			warmDur = "5s"
		}
		warmup := fmt.Sprintf(
			"echo '[vegeta] warmup target=%s url=%s'; printf %%b\\\\n %s | vegeta attack -rate=%s -duration=%s > /dev/null",
			target.Name,
			target.URL,
			targetLineEscaped,
			shellEscape(warmRate),
			shellEscape(warmDur),
		)
		return warmup + "; " + mainAttack
	}
	return mainAttack
}

func buildAttackConfig(global loadtestv1alpha1.AttackSpec, override *loadtestv1alpha1.AttackOverrideSpec) attackConfig {
	cfg := attackConfig{
		Rate:         defaultString(global.Rate, "50/1s"),
		Duration:     defaultString(global.Duration, "10s"),
		Timeout:      defaultString(global.Timeout, "30s"),
		Workers:      defaultInt32(global.Workers, 10),
		MaxWorkers:   global.MaxWorkers,
		MaxBody:      defaultInt64(global.MaxBody, -1),
		KeepAlive:    defaultBool(global.KeepAlive, true),
		HTTP2:        defaultBool(global.HTTP2, true),
		H2C:          global.H2C,
		Insecure:     global.Insecure,
		Redirects:    defaultInt32(global.Redirects, 10),
		Chunked:      global.Chunked,
		Connections:  defaultInt32(global.Connections, 10000),
		UnixSocket:   global.UnixSocket,
		ProxyURL:     global.ProxyURL,
		LocalAddress: global.LocalAddress,
		Resolvers:    global.Resolvers,
	}
	if override != nil {
		if override.Rate != "" {
			cfg.Rate = override.Rate
		}
		if override.Duration != "" {
			cfg.Duration = override.Duration
		}
		if override.Timeout != "" {
			cfg.Timeout = override.Timeout
		}
	}
	return cfg
}

func buildAttackFlags(cfg attackConfig) string {
	flags := []string{
		"-rate=" + shellEscape(cfg.Rate),
		"-duration=" + shellEscape(cfg.Duration),
		"-timeout=" + shellEscape(cfg.Timeout),
		fmt.Sprintf("-workers=%d", cfg.Workers),
		fmt.Sprintf("-max-body=%d", cfg.MaxBody),
		fmt.Sprintf("-redirects=%d", cfg.Redirects),
		fmt.Sprintf("-connections=%d", cfg.Connections),
	}
	if cfg.MaxWorkers > 0 {
		flags = append(flags, fmt.Sprintf("-max-workers=%d", cfg.MaxWorkers))
	}
	if !cfg.KeepAlive {
		flags = append(flags, "-keepalive=false")
	}
	if !cfg.HTTP2 {
		flags = append(flags, "-http2=false")
	}
	if cfg.H2C {
		flags = append(flags, "-h2c=true")
	}
	if cfg.Insecure {
		flags = append(flags, "-insecure=true")
	}
	if cfg.Chunked {
		flags = append(flags, "-chunked=true")
	}
	if cfg.UnixSocket != "" {
		flags = append(flags, "-unix-socket="+shellEscape(cfg.UnixSocket))
	}
	if cfg.ProxyURL != "" {
		flags = append(flags, "-proxy-header="+shellEscape(cfg.ProxyURL))
	}
	if cfg.LocalAddress != "" {
		flags = append(flags, "-laddr="+shellEscape(cfg.LocalAddress))
	}
	if len(cfg.Resolvers) > 0 {
		flags = append(flags, "-resolvers="+shellEscape(strings.Join(cfg.Resolvers, ",")))
	}
	return strings.Join(flags, " ")
}

func resolveImage(img *loadtestv1alpha1.ImageSpec) string {
	repo := "docker.io/peterevans/vegeta"
	tag := "latest"
	if img != nil {
		if img.Repository != "" {
			repo = img.Repository
		}
		if img.Tag != "" {
			tag = img.Tag
		}
	}
	return repo + ":" + tag
}

func resolvePullPolicy(img *loadtestv1alpha1.ImageSpec) corev1.PullPolicy {
	if img != nil && img.PullPolicy != "" {
		return img.PullPolicy
	}
	return corev1.PullIfNotPresent
}

func buildScheme(cr *loadtestv1alpha1.VegetaLoadTest) *runtime.Scheme {
	s := runtime.NewScheme()
	_ = loadtestv1alpha1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = cr
	return s
}

func methodOrDefault(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func defaultInt32(v, d int32) int32 {
	if v == 0 {
		return d
	}
	return v
}

func defaultInt64(v, d int64) int64 {
	if v == 0 {
		return d
	}
	return v
}

func defaultBool(v, d bool) bool {
	if !v {
		return d
	}
	return v
}

func shellEscape(v string) string {
	return strings.ReplaceAll(v, " ", "\\ ")
}

func int32Ptr(v int32) *int32 { return &v }
