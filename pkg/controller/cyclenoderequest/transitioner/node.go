package transitioner

import (
	"fmt"

	v1 "github.com/atlassian-labs/cyclops/pkg/apis/atlassian/v1"
	"github.com/atlassian-labs/cyclops/pkg/cloudprovider"
	corev1 "k8s.io/api/core/v1"
)

// listReadyNodes lists nodes that are "ready". By default lists nodes that have also not been touched by Cyclops.
// A label is used to determine whether nodes have been touched by this CycleNodeRequest.
func (t *CycleNodeRequestTransitioner) listReadyNodes(includeInProgress bool) (nodes []corev1.Node, err error) {
	// Get the nodes
	selector, err := t.cycleNodeRequest.NodeLabelSelector()
	if err != nil {
		return nodes, err
	}
	nodeList, err := t.rm.ListNodes(selector)
	if err != nil {
		return nodes, err
	}

	// Filter the nodes down
	for _, node := range nodeList {
		if !includeInProgress {
			// Only add nodes that are not in progress
			if value, ok := node.Labels[cycleNodeLabel]; ok && value == t.cycleNodeRequest.Name {
				continue
			}
		}
		// Only add "Ready" nodes
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				nodes = append(nodes, node)
				break
			}
		}
	}
	return nodes, nil
}

// getNodesToTerminate returns a list of nodes that still need terminating and have not yet been actioned for
// this CycleNodeRequest.
// Also returns the number of nodes currently being cycled that still exist in the cluster.
func (t *CycleNodeRequestTransitioner) getNodesToTerminate(numNodes int64) (nodes []*corev1.Node, numNodesInProgress int, err error) {
	if numNodes < 0 {
		return nil, 0, fmt.Errorf("numNodes must be positive: got %d", numNodes)
	}

	// We have to include in progress nodes so we can count them
	kubeNodes, err := t.listReadyNodes(true)
	if err != nil {
		return nil, 0, err
	}

	for _, nodeToTerminate := range t.cycleNodeRequest.Status.NodesToTerminate {
		for _, kubeNode := range kubeNodes {
			// Skip nodes that are already being worked on so we don't duplicate our work
			if value, ok := kubeNode.Labels[cycleNodeLabel]; ok && value == t.cycleNodeRequest.Name {
				numNodesInProgress++
				break
			}

			// Add nodes that need to be terminated but have not yet been actioned
			if kubeNode.Name == nodeToTerminate.Name && kubeNode.Spec.ProviderID == nodeToTerminate.ProviderID {
				nodes = append(nodes, &kubeNode)

				for i := 0; i < len(t.cycleNodeRequest.Status.NodesAvailable); i++ {
					if kubeNode.Name == t.cycleNodeRequest.Status.NodesAvailable[i].Name {
						// Remove nodes from available if they are also scheduled for termination
						// Slice syntax removes this node at `i` from the array
						t.cycleNodeRequest.Status.NodesAvailable = append(
							t.cycleNodeRequest.Status.NodesAvailable[:i],
							t.cycleNodeRequest.Status.NodesAvailable[i+1:]...,
						)

						break
					}
				}

				break
			}
		}

		// Stop finding nodes once we reach the desired amount
		if int64(len(nodes)) >= numNodes {
			break
		}
	}

	return nodes, numNodesInProgress, nil
}

// addNamedNodesToTerminate adds the named nodes for this CycleNodeRequest to the list of nodes to terminate.
// Returns an error if any named node does not exist in the node group for this CycleNodeRequest.
func (t *CycleNodeRequestTransitioner) addNamedNodesToTerminate(kubeNodes []corev1.Node, nodeGroupInstances map[string]cloudprovider.Instance) error {
	for _, namedNode := range t.cycleNodeRequest.Spec.NodeNames {
		foundNode := false
		for _, kubeNode := range kubeNodes {
			if kubeNode.Name == namedNode {
				foundNode = true

				t.cycleNodeRequest.Status.NodesAvailable = append(
					t.cycleNodeRequest.Status.NodesAvailable,
					newCycleNodeRequestNode(&kubeNode, nodeGroupInstances[kubeNode.Spec.ProviderID].NodeGroupName()),
				)

				t.cycleNodeRequest.Status.NodesToTerminate = append(
					t.cycleNodeRequest.Status.NodesToTerminate,
					newCycleNodeRequestNode(&kubeNode, nodeGroupInstances[kubeNode.Spec.ProviderID].NodeGroupName()),
				)

				break
			}
		}

		if !foundNode {
			return fmt.Errorf("could not find node by name: %v", namedNode)
		}
	}
	return nil
}

// newCycleNodeRequestNode converts a corev1.Node to a v1.CycleNodeRequestNode. This is done multiple
// times across the code, this function standardises the process
func newCycleNodeRequestNode(kubeNode *corev1.Node, nodeGroupName string) v1.CycleNodeRequestNode {
	var privateIP string

	// If there is no private IP, the error will be caught when trying
	// to perform a health check on the node
	for _, address := range kubeNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			privateIP = address.Address
		}
	}

	return v1.CycleNodeRequestNode{
		Name:          kubeNode.Name,
		ProviderID:    kubeNode.Spec.ProviderID,
		NodeGroupName: nodeGroupName,
		PrivateIP:     privateIP,
	}
}
