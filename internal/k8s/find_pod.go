package k8s

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FindPod(ctx context.Context, cfg *ClientConfig, ns, labelSelector, fieldSelector string) ([]coreV1.Pod, error) {
	opts := v1.ListOptions{}
	if labelSelector != "" {
		opts.LabelSelector = labelSelector
	}
	if fieldSelector != "" {
		opts.FieldSelector = fieldSelector
	}
	pods, err := cfg.Clientset.CoreV1().Pods(ns).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

