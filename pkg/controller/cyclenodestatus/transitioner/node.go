package transitioner

import (
	"github.com/atlassian-labs/cyclops/pkg/k8s"
	v1 "k8s.io/api/core/v1"
)

// removeLabelsFromNodes makes sure a list of labels removed from the node
func (t *CycleNodeStatusTransitioner) removeLabelsFromNodes() (finished bool, err error) {
	if len(t.cycleNodeStatus.Spec.CycleSettings.DaemonsetPodsToRemove.NodeLabelsToRemove) == 0 {
		return true, nil
	}

	// get node object
	node, err := t.rm.GetNode(t.cycleNodeStatus.Status.CurrentNode.Name)
	if err != nil {
		return finished, err
	}
	// remove any matching labels from the node
	labelsRemoved := 0
	var labelsToRemove []string
	// Check to see if the node has labels inside the list
	for label := range t.cycleNodeStatus.Spec.CycleSettings.DaemonsetPodsToRemove.NodeLabelsToRemove {
		if _, ok := node.Labels[label]; ok {
			labelsToRemove = append(labelsToRemove, label)
		}
	}

	// Remove the labels by patching the node
	if len(labelsToRemove) > 0 {
		labelsRemoved += len(labelsToRemove)
		if err := k8s.RemoveLabelsFromNode(node.Name, labelsToRemove, t.rm.RawClient); err != nil {
			return finished, err
		}
	}

	return labelsRemoved == 0, nil
}

// waitTargetedDaemonsetPodsRemoved checks if all targeted daemonsets pods have been removed from the node
func (t *CycleNodeStatusTransitioner) waitTargetedDaemonsetPodsRemoved() (finished bool, err error) {
	// List all pods on the node
	pods, err := t.rm.GetPodsOnNode(t.cycleNodeStatus.Status.CurrentNode.Name)
	if err != nil {
		return finished, err
	}
	targetedPods := make([]v1.Pod, 0, len(pods))
	for _, pod := range pods {
		if !k8s.PodIsDaemonSet(&pod) {
			continue
		}
		// make sure daemonset pods contain target pod labels
		if !podContainsLabel(&pod, t.cycleNodeStatus.Spec.CycleSettings.DaemonsetPodsToRemove.DaemonsetPodsLabelsToWait) {
			continue
		}
		// make sure daemonset pods container target node selector
		if !podHasNodeSelector(&pod, t.cycleNodeStatus.Spec.CycleSettings.DaemonsetPodsToRemove.NodeLabelsToRemove) {
			continue
		}
		targetedPods = append(targetedPods, pod)
	}

	return len(targetedPods) == 0, nil
}

// podContainsLabel validates if a pod contains any of the labels passed in
func podContainsLabel(pod *v1.Pod, labels map[string]string) bool {
	for key, value := range labels {
		if v, ok := pod.Labels[key]; ok {
			if value == v {
				return true
			}
		}
	}
	return false
}

// podHasNodeSelector validates if a pod has any node selector inside the selector passed in
func podHasNodeSelector(pod *v1.Pod, selectors map[string]string) bool {
	for selectorKey, selectorValue := range selectors {
		if v, ok := pod.Spec.NodeSelector[selectorKey]; ok {
			if v == selectorValue {
				return true
			}
		}
	}
	return false
}
