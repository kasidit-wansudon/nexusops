package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps the Kubernetes clientset with NexusOps-specific operations.
type K8sClient struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
}

// DeploymentConfig describes a Kubernetes deployment to create or update.
type DeploymentConfig struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Replicas  int32             `json:"replicas"`
	Ports     []int32           `json:"ports"`
	Env       map[string]string `json:"env"`
	Resources ResourceConfig    `json:"resources"`
	Labels    map[string]string `json:"labels"`
	Command   []string          `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
}

// ResourceConfig specifies CPU and memory requests/limits.
type ResourceConfig struct {
	CPURequest    string `json:"cpu_request"`
	CPULimit      string `json:"cpu_limit"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
}

// DeploymentStatus reports the current state of a deployment.
type DeploymentStatus struct {
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	AvailableReplicas int32              `json:"available_replicas"`
	ReadyReplicas     int32              `json:"ready_replicas"`
	UpdatedReplicas   int32              `json:"updated_replicas"`
	Replicas          int32              `json:"replicas"`
	Conditions        []DeployCondition  `json:"conditions"`
}

// DeployCondition mirrors a Kubernetes deployment condition.
type DeployCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// ServiceConfig describes a Kubernetes service to create.
type ServiceConfig struct {
	Name       string            `json:"name"`
	Ports      []ServicePort     `json:"ports"`
	Selector   map[string]string `json:"selector"`
	Type       string            `json:"type"` // ClusterIP, NodePort, LoadBalancer
	Labels     map[string]string `json:"labels"`
}

// ServicePort maps a service port to a target port.
type ServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"target_port"`
	Protocol   string `json:"protocol"`
}

// NewK8sClient creates a client from a kubeconfig file path.
func NewK8sClient(kubeconfig string) (*K8sClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("k8s: building config from %s: %w", kubeconfig, err)
	}
	return newFromConfig(config)
}

// NewInClusterClient creates a client using the in-cluster service account.
func NewInClusterClient() (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("k8s: in-cluster config: %w", err)
	}
	return newFromConfig(config)
}

func newFromConfig(config *rest.Config) (*K8sClient, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("k8s: creating clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("k8s: creating dynamic client: %w", err)
	}

	return &K8sClient{
		clientset:     clientset,
		dynamicClient: dynClient,
		config:        config,
	}, nil
}

// CreateDeployment creates a new deployment in the given namespace.
func (k *K8sClient) CreateDeployment(ctx context.Context, namespace string, cfg *DeploymentConfig) error {
	deployment := buildDeployment(cfg)
	_, err := k.clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("k8s: creating deployment %s: %w", cfg.Name, err)
	}
	return nil
}

// UpdateDeployment applies changes to an existing deployment.
func (k *K8sClient) UpdateDeployment(ctx context.Context, namespace string, cfg *DeploymentConfig) error {
	deployment := buildDeployment(cfg)
	_, err := k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("k8s: updating deployment %s: %w", cfg.Name, err)
	}
	return nil
}

// DeleteDeployment removes a deployment by name.
func (k *K8sClient) DeleteDeployment(ctx context.Context, namespace, name string) error {
	propagation := metav1.DeletePropagationForeground
	err := k.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("k8s: deleting deployment %s: %w", name, err)
	}
	return nil
}

// GetDeploymentStatus returns the current status of a deployment.
func (k *K8sClient) GetDeploymentStatus(ctx context.Context, namespace, name string) (*DeploymentStatus, error) {
	deploy, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8s: getting deployment %s: %w", name, err)
	}

	conditions := make([]DeployCondition, 0, len(deploy.Status.Conditions))
	for _, c := range deploy.Status.Conditions {
		conditions = append(conditions, DeployCondition{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}

	return &DeploymentStatus{
		Name:              deploy.Name,
		Namespace:         deploy.Namespace,
		AvailableReplicas: deploy.Status.AvailableReplicas,
		ReadyReplicas:     deploy.Status.ReadyReplicas,
		UpdatedReplicas:   deploy.Status.UpdatedReplicas,
		Replicas:          deploy.Status.Replicas,
		Conditions:        conditions,
	}, nil
}

// ScaleDeployment changes the replica count for a deployment.
func (k *K8sClient) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
	_, err := k.clientset.AppsV1().Deployments(namespace).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("k8s: scaling deployment %s to %d: %w", name, replicas, err)
	}
	return nil
}

// CreateService creates a Kubernetes service in the given namespace.
func (k *K8sClient) CreateService(ctx context.Context, namespace string, cfg *ServiceConfig) error {
	svc := buildService(cfg)
	_, err := k.clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("k8s: creating service %s: %w", cfg.Name, err)
	}
	return nil
}

// DeleteService removes a service by name.
func (k *K8sClient) DeleteService(ctx context.Context, namespace, name string) error {
	err := k.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("k8s: deleting service %s: %w", name, err)
	}
	return nil
}

// ApplyManifest applies a raw YAML/JSON manifest using server-side apply via
// the dynamic client.
func (k *K8sClient) ApplyManifest(ctx context.Context, namespace string, manifest []byte) error {
	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	obj := &unstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(manifest, nil, obj)
	if err != nil {
		return fmt.Errorf("k8s: decoding manifest: %w", err)
	}

	if namespace != "" {
		obj.SetNamespace(namespace)
	}

	// Discover the API resource for the GVK.
	mapping, err := k.clientset.Discovery().ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return fmt.Errorf("k8s: discovering resource for %s: %w", gvk.String(), err)
	}

	var gvr string
	for _, r := range mapping.APIResources {
		if r.Kind == gvk.Kind {
			gvr = r.Name
			break
		}
	}
	if gvr == "" {
		return fmt.Errorf("k8s: no resource found for kind %s", gvk.Kind)
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("k8s: marshalling object: %w", err)
	}

	resource := gvk.GroupVersion().WithResource(gvr)
	_, err = k.dynamicClient.Resource(resource).Namespace(namespace).Patch(
		ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
			FieldManager: "nexusops",
		},
	)
	if err != nil {
		return fmt.Errorf("k8s: applying manifest for %s/%s: %w", gvk.Kind, obj.GetName(), err)
	}

	return nil
}

// --- builders ---

func buildDeployment(cfg *DeploymentConfig) *appsv1.Deployment {
	labels := cfg.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels["app.kubernetes.io/name"] = cfg.Name
	labels["app.kubernetes.io/managed-by"] = "nexusops"

	// Container ports.
	containerPorts := make([]corev1.ContainerPort, 0, len(cfg.Ports))
	for _, p := range cfg.Ports {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: p,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Env vars.
	envVars := make([]corev1.EnvVar, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Resources.
	resources := corev1.ResourceRequirements{}
	if cfg.Resources.CPURequest != "" || cfg.Resources.MemoryRequest != "" {
		resources.Requests = corev1.ResourceList{}
		if cfg.Resources.CPURequest != "" {
			resources.Requests[corev1.ResourceCPU] = resource.MustParse(cfg.Resources.CPURequest)
		}
		if cfg.Resources.MemoryRequest != "" {
			resources.Requests[corev1.ResourceMemory] = resource.MustParse(cfg.Resources.MemoryRequest)
		}
	}
	if cfg.Resources.CPULimit != "" || cfg.Resources.MemoryLimit != "" {
		resources.Limits = corev1.ResourceList{}
		if cfg.Resources.CPULimit != "" {
			resources.Limits[corev1.ResourceCPU] = resource.MustParse(cfg.Resources.CPULimit)
		}
		if cfg.Resources.MemoryLimit != "" {
			resources.Limits[corev1.ResourceMemory] = resource.MustParse(cfg.Resources.MemoryLimit)
		}
	}

	replicas := cfg.Replicas
	if replicas < 1 {
		replicas = 1
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cfg.Name,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": cfg.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      cfg.Name,
							Image:     cfg.Image,
							Ports:     containerPorts,
							Env:       envVars,
							Resources: resources,
							Command:   cfg.Command,
							Args:      cfg.Args,
						},
					},
				},
			},
		},
	}
}

func buildService(cfg *ServiceConfig) *corev1.Service {
	labels := cfg.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels["app.kubernetes.io/managed-by"] = "nexusops"

	svcType := corev1.ServiceTypeClusterIP
	switch cfg.Type {
	case "NodePort":
		svcType = corev1.ServiceTypeNodePort
	case "LoadBalancer":
		svcType = corev1.ServiceTypeLoadBalancer
	case "ExternalName":
		svcType = corev1.ServiceTypeExternalName
	}

	ports := make([]corev1.ServicePort, 0, len(cfg.Ports))
	for _, p := range cfg.Ports {
		protocol := corev1.ProtocolTCP
		if p.Protocol == "UDP" {
			protocol = corev1.ProtocolUDP
		}
		ports = append(ports, corev1.ServicePort{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: intstr.FromInt32(p.TargetPort),
			Protocol:   protocol,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   cfg.Name,
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Ports:    ports,
			Selector: cfg.Selector,
		},
	}
}
