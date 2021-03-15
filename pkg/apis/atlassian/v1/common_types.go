package v1

// CycleNodeRequestMethod is the method to use when cycling nodes.
type CycleNodeRequestMethod string

const (
	// CycleNodeRequestMethodDrain actively drains pods off the node before terminating it.
	// This is the default method.
	CycleNodeRequestMethodDrain = "Drain"

	// CycleNodeRequestMethodWait waits for pods to leave the node before terminating it.
	// It will ignore DaemonSets and select pods. These can be configured in the CRD spec.
	CycleNodeRequestMethodWait = "Wait"
)

// CycleSettings are configuration options to control how nodes are cycled
// +k8s:openapi-gen=true
type CycleSettings struct {
	// Method describes the type of cycle operation to use.
	// +kubebuilder:validation:Enum=Drain;Wait
	Method CycleNodeRequestMethod `json:"method"`

	// Concurrency is the number of nodes that one CycleNodeRequest will work on in parallel.
	// Defaults to the size of the node group.
	Concurrency int64 `json:"concurrency,omitempty"`

	// LabelsToRemove is an array of labels to remove off of the pods running on the node
	// This can be used to remove a pod from a service/endpoint before evicting/deleting
	// it to prevent traffic being sent to it.
	LabelsToRemove []string `json:"labelsToRemove,omitempty"`

	// DaemonsetPodsToRemove is a struct which includes information about daemonset pods to remove
	DaemonsetPodsToRemove DaemonsetPodsToRemove `json:"daemonsetPodsToRemove,omitempty"`
	// IgnorePodLabels is a map of values for labels that describes which pods should be ignored when
	// deciding whether a node has no more pods running. This map defines a union: any pod that
	// matches any of the values for a given label name will be ignored.
	IgnorePodsLabels map[string][]string `json:"ignorePodsLabels,omitempty"`

	// IgnoreNamespaces is a list of namespace names in which running pods should be ignored
	// when deciding whether a node has no more pods running.
	IgnoreNamespaces []string `json:"ignoreNamespaces,omitempty"`
}

// DaemonsetPodsToRemove includes information about daeonset pods to remove
type DaemonsetPodsToRemove struct {
	NodeLabelsToRemove        map[string]string `json:"nodeLabelsToRemove,omitempty"`
	DaemonsetPodsLabelsToWait map[string]string `json:"daemonsetPodsLabelsToWait,omitempty"`
}
