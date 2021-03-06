/*
Copyright 2020 The Tekton Authors

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

package pipelinerun

import (
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/system"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)

// TestCreateAndDeleteOfAffinityAssistant tests to create and delete an Affinity Assistant
// for a given PipelineRun with a PVC workspace
func TestCreateAndDeleteOfAffinityAssistant(t *testing.T) {
	c := Reconciler{
		Base: &reconciler.Base{
			KubeClientSet: fakek8s.NewSimpleClientset(),
			Images:        pipeline.Images{},
			Logger:        zap.NewExample().Sugar(),
		},
	}

	workspaceName := "testws"
	pipelineRunName := "pipelinerun-1"
	testPipelineRun := &v1beta1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: pipelineRunName,
		},
		Spec: v1beta1.PipelineRunSpec{
			Workspaces: []v1beta1.WorkspaceBinding{{
				Name: workspaceName,
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "myclaim",
				},
			}},
		},
	}

	err := c.createAffinityAssistants(testPipelineRun.Spec.Workspaces, testPipelineRun, testPipelineRun.Namespace)
	if err != nil {
		t.Errorf("unexpected error from createAffinityAssistants: %v", err)
	}

	expectedAffinityAssistantName := affinityAssistantStatefulSetNamePrefix + fmt.Sprintf("%s-%s", workspaceName, pipelineRunName)
	_, err = c.KubeClientSet.AppsV1().StatefulSets(testPipelineRun.Namespace).Get(expectedAffinityAssistantName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("unexpected error when retrieving StatefulSet: %v", err)
	}

	err = c.cleanupAffinityAssistants(testPipelineRun)
	if err != nil {
		t.Errorf("unexpected error from cleanupAffinityAssistants: %v", err)
	}

	_, err = c.KubeClientSet.AppsV1().StatefulSets(testPipelineRun.Namespace).Get(expectedAffinityAssistantName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected a NotFound response, got: %v", err)
	}
}

func TestDisableAffinityAssistant(t *testing.T) {
	for _, tc := range []struct {
		description string
		configMap   *corev1.ConfigMap
		expected    bool
	}{{
		description: "Default behaviour: A missing disable-affinity-assistant flag should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: pod.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data:       map[string]string{},
		},
		expected: false,
	}, {
		description: "Setting disable-affinity-assistant to false should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: pod.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableAffinityAssistantKey: "false",
			},
		},
		expected: false,
	}, {
		description: "Setting disable-affinity-assistant to true should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: pod.GetFeatureFlagsConfigName(), Namespace: system.GetNamespace()},
			Data: map[string]string{
				featureFlagDisableAffinityAssistantKey: "true",
			},
		},
		expected: true,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			c := Reconciler{
				Base: &reconciler.Base{
					KubeClientSet: fakek8s.NewSimpleClientset(
						tc.configMap,
					),
					Images: pipeline.Images{},
					Logger: zap.NewExample().Sugar(),
				},
			}
			if result := c.isAffinityAssistantDisabled(); result != tc.expected {
				t.Errorf("Expected %t Received %t", tc.expected, result)
			}
		})
	}
}
