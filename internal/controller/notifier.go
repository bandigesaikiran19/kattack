package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	loadtestv1alpha1 "github.com/loadtest-io/kattack/api/v1alpha1"
)

type VegetaLoadTest = loadtestv1alpha1.VegetaLoadTest

func SendNotification(ctx context.Context, cr *VegetaLoadTest, c client.Client) error {
	if cr == nil || cr.Spec.Notifications == nil {
		return nil
	}

	l := log.FromContext(ctx)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	var errs []error

	if cr.Spec.Notifications.Slack != nil && cr.Spec.Notifications.Slack.WebhookSecretRef != nil {
		webhook, err := fetchSecretValue(ctx, c, cr.Namespace, cr.Spec.Notifications.Slack.WebhookSecretRef)
		if err != nil {
			errs = append(errs, fmt.Errorf("slack secret: %w", err))
		} else {
			text := buildSummaryMessage(cr)
			if err := postJSON(httpClient, webhook, map[string]string{"text": text}); err != nil {
				l.Error(err, "slack notification failed")
				errs = append(errs, err)
			}
		}
	}

	if cr.Spec.Notifications.Webhook != nil && cr.Spec.Notifications.Webhook.URL != "" {
		method := cr.Spec.Notifications.Webhook.Method
		if method == "" {
			method = http.MethodPost
		}
		body := cr.Spec.Notifications.Webhook.TemplateBody
		if body == "" {
			body = `{"name":"{{.Name}}","phase":"{{.Phase}}","thresholdsPassed":{{.ThresholdsPassed}}}`
		}
		tpl, err := template.New("webhook").Parse(body)
		if err != nil {
			errs = append(errs, fmt.Errorf("webhook template parse: %w", err))
		} else {
			data := map[string]any{
				"Name":             cr.Name,
				"Phase":            cr.Status.Phase,
				"Results":          cr.Status.Results,
				"ThresholdsPassed": cr.Status.ThresholdsPassed,
			}
			var out bytes.Buffer
			if err := tpl.Execute(&out, data); err != nil {
				errs = append(errs, fmt.Errorf("webhook template execute: %w", err))
			} else {
				req, err := http.NewRequestWithContext(ctx, method, cr.Spec.Notifications.Webhook.URL, bytes.NewBuffer(out.Bytes()))
				if err != nil {
					errs = append(errs, err)
				} else {
					for k, v := range cr.Spec.Notifications.Webhook.Headers {
						req.Header.Set(k, v)
					}
					if req.Header.Get("Content-Type") == "" {
						req.Header.Set("Content-Type", "application/json")
					}
					resp, err := httpClient.Do(req)
					if err != nil {
						l.Error(err, "webhook notification failed")
						errs = append(errs, err)
					} else {
						_ = resp.Body.Close()
						if resp.StatusCode >= 300 {
							err := fmt.Errorf("webhook notification returned status %d", resp.StatusCode)
							l.Error(err, "webhook notification failed")
							errs = append(errs, err)
						}
					}
				}
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func buildSummaryMessage(cr *VegetaLoadTest) string {
	b := &strings.Builder{}
	fmt.Fprintf(b, "VegetaLoadTest: %s\nPhase: %s\n\n", cr.Name, cr.Status.Phase)
	fmt.Fprintf(b, "Target | Phase | P99 | Throughput\n")
	for _, r := range cr.Status.Results {
		th := "n/a"
		if r.Throughput != nil {
			th = fmt.Sprintf("%.2f", *r.Throughput)
		}
		fmt.Fprintf(b, "%s | %s | %s | %s\n", r.TargetName, r.Phase, r.P99, th)
	}
	return b.String()
}

func fetchSecretValue(ctx context.Context, c client.Client, ns string, ref *loadtestv1alpha1.SecretKeyRef) (string, error) {
	if ref == nil {
		return "", errors.New("secret ref is nil")
	}
	var secret corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: ref.Name}, &secret); err != nil {
		return "", err
	}
	v, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("secret key %q not found", ref.Key)
	}
	return string(v), nil
}

func postJSON(c *http.Client, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}
	return nil
}
