package internal

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type turbineService struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Port     int32  `json:"port"`
	Replicas int32  `json:"replicas"`
	Expose   bool   `json:"expose"`
	IP       string `json:"ip"`
}

func (c turbineService) String() string {
	return fmt.Sprintf("Name: %s, Image: %s, Port: %d, Replicas: %d, Expose: %t", c.Name,
		c.Image, c.Port, c.Replicas, c.Expose)
}

type KubernetesProxy struct {
	DeploymentClient v1.DeploymentInterface
	ServiceClient    v12.ServiceInterface
	PodClient        v12.PodInterface
	NodeClient       v12.NodeInterface
}

func readAnnotation(deployment appsv1.Deployment, annotation string, defaultValue string) string {
	if val, ok := deployment.Spec.Template.Annotations[annotation]; ok {
		return val
	}
	return defaultValue
}

func (controller *KubernetesProxy) restartDeployment(ctx context.Context, deploymentName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fmt.Printf("Trying to restart deployment %s\n", deploymentName)
		result, err := controller.DeploymentClient.Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		//updating an annotation will make k8s to restart this deployment
		result.Spec.Template.Annotations["turbine/restartedAt"] = time.Now().Format(time.RFC3339)
		_, updateErr := controller.DeploymentClient.Update(ctx, result, metav1.UpdateOptions{})
		return updateErr
	})
}

func constructServiceDescriptor(applicationDescriptor turbineService) *apiv1.Service {
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationDescriptor.Name,
			Labels: map[string]string{
				"app": applicationDescriptor.Name,
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Port:     applicationDescriptor.Port,
					Protocol: apiv1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": applicationDescriptor.Name,
			},
			Type: "LoadBalancer",
		},
	}
	return service
}

func constructDeploymentDescriptor(applicationDescriptor turbineService) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationDescriptor.Name,
			Labels: map[string]string{
				"app": applicationDescriptor.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: Int32Ptr(applicationDescriptor.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": applicationDescriptor.Name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": applicationDescriptor.Name,
					},
					Annotations: map[string]string{
						"turbine/configmap": "turbine-sidecar-config",
						"turbine/enabled":   "true",
						"turbine/exposed":   strconv.FormatBool(applicationDescriptor.Expose),
						"turbine/port":      strconv.Itoa(int(applicationDescriptor.Port)),
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  applicationDescriptor.Name,
							Image: applicationDescriptor.Image,
							Ports: []apiv1.ContainerPort{
								{
									ContainerPort: applicationDescriptor.Port,
								},
							},
							ImagePullPolicy: apiv1.PullAlways,
						},
						{
							Name:  "turbine-sidecar",
							Image: "hazelcast/turbine-sidecar",
							Env: []apiv1.EnvVar{
								{
									Name: "TURBINE_POD_IP",
									ValueFrom: &apiv1.EnvVarSource{
										FieldRef: &apiv1.ObjectFieldSelector{
											FieldPath: "status.podIP",
										},
									},
								},
							},
							EnvFrom: []apiv1.EnvFromSource{
								{
									ConfigMapRef: &apiv1.ConfigMapEnvSource{
										LocalObjectReference: apiv1.LocalObjectReference{
											Name: "turbine-sidecar-config",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}

func containsContainer(deployment appsv1.Deployment, imageName string, tag string) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		nameAndTag := strings.Split(container.Image, ":")
		deploymentName := nameAndTag[0]
		deploymentTag := "latest"
		if len(nameAndTag) == 2 {
			deploymentTag = nameAndTag[1]
		}
		if deploymentName == imageName && deploymentTag == tag {
			return true
		}
	}
	return false
}

func createK8sConfig() (*rest.Config, error) {
	var kubeconfig *string
	var remote *bool
	remote = flag.Bool("remote", LookupEnvOrBoolean("remote", false), "connect to a remote cluster")

	var defaultKubeConfigPath string
	if home := homedir.HomeDir(); home != "" {
		defaultKubeConfigPath = LookupEnvOrString("kubeconfig", filepath.Join(home, ".kube", "config"))
	} else {
		defaultKubeConfigPath = LookupEnvOrString("kubeconfig", "")
	}
	kubeconfig = flag.String("kubeconfig", defaultKubeConfigPath, "absolute path to the kubeconfig file")
	flag.Parse()

	if *remote {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		return rest.InClusterConfig()
	}
}

func (controller *KubernetesProxy) listDeployment(ctx context.Context) (*appsv1.DeploymentList, error) {
	return controller.DeploymentClient.List(ctx, metav1.ListOptions{})
}

func (controller *KubernetesProxy) containsDeployment(ctx context.Context, name string) bool {
	_, err := controller.DeploymentClient.Get(ctx, name, metav1.GetOptions{})
	return err != nil
}

func (controller *KubernetesProxy) newDeployment(ctx context.Context, service turbineService) error {
	deploymentDescriptor := constructDeploymentDescriptor(service)
	_, err := controller.DeploymentClient.Create(ctx, deploymentDescriptor, metav1.CreateOptions{})
	return err
}

func (controller *KubernetesProxy) exposeService(ctx context.Context, service turbineService) error {
	descriptor := constructServiceDescriptor(service)
	_, err := controller.ServiceClient.Create(ctx, descriptor, metav1.CreateOptions{})
	return err
}

func (controller *KubernetesProxy) getAllApps(ctx context.Context) ([]turbineService, error) {
	deployments, err := controller.DeploymentClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var allTurbineApps []turbineService
	for _, deployment := range deployments.Items {
		if isTurbineApp(deployment) {
			port, err := strconv.Atoi(readAnnotation(deployment, "turbine/port", "0"))
			if err != nil {
				port = 0
			}
			exposed, err := strconv.ParseBool(readAnnotation(deployment, "turbine/exposed", "false"))
			if err != nil {
				exposed = false
			}
			var ip = ""
			if exposed {
				service, _ := controller.ServiceClient.Get(ctx, deployment.Name, metav1.GetOptions{})
				if service != nil {
					ingress := service.Status.LoadBalancer.Ingress
					if len(ingress) != 0 {
						ip = ingress[0].IP
					}
				}
			}
			turbineApp := turbineService{
				Name:     deployment.Name,
				Image:    deployment.Spec.Template.Spec.Containers[0].Image,
				Port:     int32(port),
				Replicas: *deployment.Spec.Replicas,
				Expose:   exposed,
				IP:       ip,
			}
			allTurbineApps = append(allTurbineApps, turbineApp)
		}
	}
	return allTurbineApps, nil
}

func (controller *KubernetesProxy) deleteApplication(ctx context.Context, name string) error {
	deployment, err := controller.DeploymentClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if !isTurbineApp(*deployment) {
		return errors.New(fmt.Sprintf("Application %s is not a Turbine-application", name))
	}

	serviceShouldExist := readAnnotation(*deployment, "turbine/exposed", "false")
	err = controller.DeploymentClient.Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	if serviceShouldExist == "true" {
		if err := controller.ServiceClient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func NewKubernetesProxy(namespace string) *KubernetesProxy {
	config, err := createK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return &KubernetesProxy{
		DeploymentClient: clientset.AppsV1().Deployments(namespace),
		ServiceClient:    clientset.CoreV1().Services(namespace),
		PodClient:        clientset.CoreV1().Pods(namespace),
		NodeClient:       clientset.CoreV1().Nodes(),
	}
}

func isTurbineApp(deployment appsv1.Deployment) bool {
	annotation := readAnnotation(deployment, "turbine/enabled", "false")
	return annotation == "true"
}
