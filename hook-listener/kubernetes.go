package main

import (
	"context"
	"flag"
	"fmt"
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

type ClientResources struct {
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

func restartDeployment(deploymentName string, deploymentClient v1.DeploymentInterface) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fmt.Printf("Trying to restart deployment %s\n", deploymentName)
		result, err := deploymentClient.Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		//updating an annotation will make k8s to restart this deployment
		result.Spec.Template.Annotations["turbine/restartedAt"] = time.Now().Format(time.RFC3339)
		_, updateErr := deploymentClient.Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})
}

func constructServiceDescriptor(applicationDescriptor TurbineService) *apiv1.Service {
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

func constructDeploymentDescriptor(applicationDescriptor TurbineService) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: applicationDescriptor.Name,
			Labels: map[string]string{
				"app": applicationDescriptor.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(applicationDescriptor.Replicas),
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
	remote = flag.Bool("remote", lookupEnvOrBoolean("remote", false), "connect to a remote cluster")

	var defaultKubeConfigPath string
	if home := homedir.HomeDir(); home != "" {
		defaultKubeConfigPath = lookupEnvOrString("kubeconfig", filepath.Join(home, ".kube", "config"))
	} else {
		defaultKubeConfigPath = lookupEnvOrString("kubeconfig", "")
	}
	kubeconfig = flag.String("kubeconfig", defaultKubeConfigPath, "absolute path to the kubeconfig file")
	flag.Parse()

	if *remote {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		return rest.InClusterConfig()
	}
}

func createClients(namespace string) *ClientResources {
	config, err := createK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	clusterResources = &ClientResources{
		DeploymentClient: clientset.AppsV1().Deployments(namespace),
		ServiceClient:    clientset.CoreV1().Services(namespace),
		PodClient:        clientset.CoreV1().Pods(namespace),
		NodeClient:       clientset.CoreV1().Nodes(),
	}
	return clusterResources
}

func isTurbineApp(deployment appsv1.Deployment) bool {
	annotation := readAnnotation(deployment, "turbine/enabled", "false")
	return annotation == "true"
}
