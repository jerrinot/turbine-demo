package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	"net/http"
	"path/filepath"
	"time"
)

type TurbineService struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Port     int32  `json:"port"`
	Replicas int32  `json:"replicas"`
	Expose   bool   `json:"expose"`
}

func (c TurbineService) String() string {
	return fmt.Sprintf("Name: %s, Image: %s, Port: %d, Replicas: %d, Expose: %t", c.Name,
		c.Image, c.Port, c.Replicas, c.Expose)
}

type Repository struct {
	RepoName  string `json:"repo_name"`
	RepoUrl   string `json:"repo_url"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type PushedData struct {
	Tag      string `json:"tag"`
	PushedAt uint64 `json:"pushed_at"`
}

type DockerHubEvent struct {
	CallbackUrl string     `json:"callback_url"`
	Repository  Repository `json:"repository"`
	PushedData  PushedData `json:"push_data"`
}

var deploymentClient v1.DeploymentInterface
var serviceClient v12.ServiceInterface

func handleHookRequest(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		var event DockerHubEvent
		if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, deployment := range deployments.Items {
			if isTurbineApp(deployment) && containsContainer(deployment, event.Repository.RepoName) {
				err := restartDeployment(deployment.Name, deploymentClient)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
	default:
		http.Error(w, "Unsupported HTTP method, this is POST-only", http.StatusBadRequest)
	}
}

func handleKubernetesRequest(w http.ResponseWriter, req *http.Request) {
	deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, deployment := range deployments.Items {
		if isTurbineApp(deployment) {
			err := restartDeployment(deployment.Name, deploymentClient)
			if err != nil {
				panic(err.Error())
			}
		}
	}
	fmt.Printf("There are %d deployments in the cluster\n", len(deployments.Items))
}

func handleDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	fmt.Println("I just received a request to deploy a new service")
	switch req.Method {
	case "POST":
		var applicationDescriptor TurbineService
		//todo: validate the descriptor

		if err := json.NewDecoder(req.Body).Decode(&applicationDescriptor); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Printf("Request to deploy a new application: %s\n", applicationDescriptor)
		_, err := deploymentClient.Get(context.TODO(), applicationDescriptor.Name, metav1.GetOptions{})
		if err == nil {
			http.Error(w, fmt.Sprintf("Application %s already exist", applicationDescriptor.Name), http.StatusConflict)
			return
		}
		deployment := constructDeploymentDescriptor(applicationDescriptor)
		deployment, err = deploymentClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if applicationDescriptor.Expose {
			service := constructServiceDescriptor(applicationDescriptor)
			_, err := serviceClient.Create(context.TODO(), service, metav1.CreateOptions{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	//case "GET":
	//deployments, err := deploymentClient.List(context.TODO(),  metav1.ListOptions{})

	default:
		http.Error(w, "Unsupported HTTP method, this is POST-only", http.StatusBadRequest)
	}
}

func isTurbineApp(deployment appsv1.Deployment) bool {
	val, ok := deployment.Spec.Template.Annotations["turbine/enabled"]
	return ok && "true" == val
}

func containsContainer(deployment appsv1.Deployment, containerName string) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Image == containerName {
			return true
		}
	}
	return false
}

func restartDeployment(deploymentName string, deploymentClient v1.DeploymentInterface) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fmt.Printf("Trying to restart deployment %s\n", deploymentName)
		result, err := deploymentClient.Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}

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

func handleGithubRequest(w http.ResponseWriter, req *http.Request) {
	fmt.Println("I just received a request from Github action")
}

func createK8sConfig() (*rest.Config, error) {
	var kubeconfig *string
	var remote *bool
	remote = flag.Bool("remote", false, "connect to a remote cluster")

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	if *remote {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		return rest.InClusterConfig()
	}
}

func createClients(namespace string) (v1.DeploymentInterface, v12.ServiceInterface) {
	config, err := createK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	deploymentClient := clientset.AppsV1().Deployments(namespace)
	serviceClient := clientset.CoreV1().Services(namespace)

	return deploymentClient, serviceClient
}

func int32Ptr(i int32) *int32 { return &i }

func main() {
	deploymentClient, serviceClient = createClients("default")

	http.HandleFunc("/webhook", handleHookRequest)
	http.HandleFunc("/k8s", handleKubernetesRequest)
	http.HandleFunc("/gh-action", handleGithubRequest)
	http.HandleFunc("/deployment", handleDeploymentRequest)
	http.ListenAndServe(":8080", nil)
}
