// Copyright (c) 2020 TypeFox GmbH. All rights reserved.
// Licensed under the Gitpod Enterprise Source Code License,
// See License.enterprise.txt in the project root folder.

package scheduler_test

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	res "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sched "github.com/gitpod-io/gitpod/ws-scheduler/pkg/scheduler"
)

var (
	testBaseTime       = time.Unix(0, 0)
	testWorkspaceImage = "gitpod/workspace-full"
)

func TestDensityAndExperience(t *testing.T) {
	tests := []struct {
		Desc          string
		Broken        string
		Nodes         []*corev1.Node
		Pods          []*corev1.Pod
		ScheduledPod  *corev1.Pod
		ExpectedNode  string
		ExpectedError string
	}{
		{
			Desc:         "no node",
			ScheduledPod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "testpod"}},
			ExpectedError: `No node with enough resources available!
RAM requested: 0
Eph. Storage requested: 0
Nodes:
`,
		},
		{
			Desc:         "no node with enough RAM",
			Nodes:        []*corev1.Node{createNode("node1", "10Gi", "0Gi", false, 100)},
			Pods:         []*corev1.Pod{createNonWorkspacePod("existingPod1", "8Gi", "0Gi", "node1", 10)},
			ScheduledPod: createWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedError: `No node with enough resources available!
RAM requested: 6Gi
Eph. Storage requested: 0
Nodes:
- node1:
  RAM: used 0+0+8Gi of 10Gi, avail 2Gi
  Eph. Storage: used 0+0+0 of 0, avail 0`,
		},
		{
			Desc:         "single empty node",
			Nodes:        []*corev1.Node{createNode("node1", "10Gi", "0Gi", false, 100)},
			ScheduledPod: createWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node1",
		},
		{
			Desc: "two nodes, one full",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", false, 100),
			},
			Pods:         []*corev1.Pod{createNonWorkspacePod("existingPod1", "8Gi", "0Gi", "node1", 10)},
			ScheduledPod: createWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node2",
		},
		{
			Desc: "two nodes, prefer density",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", false, 100),
			},
			Pods:         []*corev1.Pod{createWorkspacePod("existingPod1", "1Gi", "0Gi", "node1", 10)},
			ScheduledPod: createWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node1",
		},
		{
			Desc: "three nodes, prefer with image",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", true, 100),
				createNode("node3", "10Gi", "0Gi", false, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "1.5Gi", "0Gi", "node1", 10),
				createWorkspacePod("existingPod2", "1Gi", "0Gi", "node2", 10),
			},
			ScheduledPod: createWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node2",
		},
		{
			Desc: "three nodes, prefer with image in class",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", false, 100),
				createNode("node3", "10Gi", "0Gi", true, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "1.5Gi", "0Gi", "node1", 10),
				createWorkspacePod("existingPod2", "1Gi", "0Gi", "node2", 10),
			},
			ScheduledPod: createWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node1",
		},
		{
			// We musn't place headless pods on nodes without regular workspaces
			Desc: "three nodes, place headless pod",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", true, 100),
				createNode("node3", "10Gi", "0Gi", true, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "1.5Gi", "0Gi", "node1", 10),
				createWorkspacePod("existingPod2", "1Gi", "0Gi", "node2", 10),
				createHeadlessWorkspacePod("hpod", "0.5Gi", "0Gi", "node3", 1000),
			},
			ScheduledPod: createHeadlessWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node2",
		},
		{
			Desc: "three empty nodes, place headless pod",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", true, 100),
				createNode("node3", "10Gi", "0Gi", true, 100),
			},
			ScheduledPod: createHeadlessWorkspacePod("pod", "6Gi", "0Gi", "", 1000),
			ExpectedNode: "node1",
		},
		{
			Desc: "filter full nodes, headless workspaces",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 100),
				createNode("node2", "10Gi", "0Gi", false, 100),
			},
			Pods: []*corev1.Pod{
				createHeadlessWorkspacePod("existingPod1", "4Gi", "0Gi", "node1", 10),
				createWorkspacePod("existingPod2", "4Gi", "0Gi", "node1", 10),
			},
			ScheduledPod: createWorkspacePod("pod", "4Gi", "0Gi", "", 10),
			ExpectedNode: "node2",
		},
		{
			// Should choose node1 because it has more free RAM but chooses 2 because node1's pod capacity is depleted
			Desc: "respect node's pod capacity",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "0Gi", false, 0),
				createNode("node2", "10Gi", "0Gi", false, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "4Gi", "0Gi", "node2", 10),
			},
			ScheduledPod: createWorkspacePod("new pod", "4Gi", "0Gi", "node1", 10),
			ExpectedNode: "node2",
		},
		{
			// Should choose node1 because it has more free RAM but chooses 2 because node1's ephemeral storage is depleted
			Desc: "respect node's ephemeral storage",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "3Gi", false, 100),
				createNode("node2", "10Gi", "15Gi", false, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "4Gi", "5Gi", "node2", 10),
			},
			ScheduledPod: createWorkspacePod("new pod", "4Gi", "5Gi", "node1", 10),
			ExpectedNode: "node2",
		},
		{
			// Throws an error because both nodes have enough RAM but not enough ephemeral storage
			Desc: "enough RAM but no more ephemeral storage",
			Nodes: []*corev1.Node{
				createNode("node1", "10Gi", "3Gi", false, 100),
				createNode("node2", "10Gi", "7Gi", false, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "4Gi", "5Gi", "node2", 10),
			},
			ScheduledPod: createWorkspacePod("new pod", "4Gi", "5Gi", "node1", 10),
			ExpectedError: `No node with enough resources available!
RAM requested: 4Gi
Eph. Storage requested: 5Gi
Nodes:
- node2:
  RAM: used 4Gi+0+0 of 10Gi, avail 6Gi
  Eph. Storage: used 5Gi+0+0 of 7Gi, avail 2Gi
- node1:
  RAM: used 0+0+0 of 10Gi, avail 10Gi
  Eph. Storage: used 0+0+0 of 3Gi, avail 3Gi`,
		},
		{
			// Should prefer 1 and 2 over 3, but 1 has not enough pod slots and 2 not enough ephemeral storage
			Desc: "filter nodes without enough pod slots and ephemeral storage",
			Nodes: []*corev1.Node{
				createNode("node1", "20Gi", "10Gi", false, 0),
				createNode("node2", "20Gi", "10Gi", false, 100),
				createNode("node3", "20Gi", "10Gi", false, 100),
			},
			Pods: []*corev1.Pod{
				createWorkspacePod("existingPod1", "4Gi", "5Gi", "node2", 10),
				createWorkspacePod("existingPod2", "4Gi", "5Gi", "node2", 10),
				createWorkspacePod("existingPod3", "4Gi", "5Gi", "node3", 10),
			},
			ScheduledPod: createWorkspacePod("new pod", "4Gi", "5Gi", "node1", 10),
			ExpectedNode: "node3",
		},
	}

	for _, test := range tests {
		t.Run(test.Desc, func(t *testing.T) {
			if test.Broken != "" {
				t.Skip(test.Broken)
			}

			state := sched.ComputeState(test.Nodes, test.Pods, nil)

			densityAndExperienceConfig := sched.DefaultDensityAndExperienceConfig()
			strategy, err := sched.CreateStrategy(sched.StrategyDensityAndExperience, sched.Configuration{
				DensityAndExperienceConfig: densityAndExperienceConfig,
			})
			if err != nil {
				t.Errorf("cannot create strategy: %v", err)
				return
			}

			node, err := strategy.Select(state, test.ScheduledPod)
			var errmsg string
			if err != nil {
				errmsg = err.Error()
			}
			if errmsg != test.ExpectedError {
				t.Errorf("expected error \"%s\", got \"%s\"", test.ExpectedError, errmsg)
				return
			}
			if node != test.ExpectedNode {
				t.Errorf("expected node \"%s\", got \"%s\"", test.ExpectedNode, node)
				return
			}
		})
	}
}

func createNode(name string, ram string, ephemeralStorage string, withImage bool, podCapacity int64) *corev1.Node {
	images := make([]corev1.ContainerImage, 0)
	if withImage {
		images = append(images, corev1.ContainerImage{
			Names: []string{testWorkspaceImage},
		})
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory:           res.MustParse(ram),
				corev1.ResourceEphemeralStorage: res.MustParse(ephemeralStorage),
			},
			Images: images,
			Capacity: corev1.ResourceList{
				corev1.ResourcePods: *res.NewQuantity(podCapacity, res.DecimalSI),
			},
		},
	}
}

func createNonWorkspacePod(name string, ram string, ephemeralStorage string, nodeName string, age time.Duration) *corev1.Pod {
	return createPod(name, ram, ephemeralStorage, nodeName, age, map[string]string{})
}

func createHeadlessWorkspacePod(name string, ram string, ephemeralStorage string, nodeName string, age time.Duration) *corev1.Pod {
	return createPod(name, ram, ephemeralStorage, nodeName, age, map[string]string{
		"component": "workspace",
		"headless":  "true",
	})
}

func createWorkspacePod(name string, ram string, ephemeralStorage string, nodeName string, age time.Duration) *corev1.Pod {
	return createPod(name, ram, ephemeralStorage, nodeName, age, map[string]string{
		"component": "workspace",
	})
}

func createPod(name string, ram string, ephemeralStorage string, nodeName string, age time.Duration, labels map[string]string) *corev1.Pod {
	creationTimestamp := testBaseTime.Add(age * time.Second)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(creationTimestamp),
			Labels:            labels,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:  "workspace",
					Image: testWorkspaceImage,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory:           res.MustParse(ram),
							corev1.ResourceEphemeralStorage: res.MustParse(ephemeralStorage),
						},
					},
				},
			},
		},
	}
}
