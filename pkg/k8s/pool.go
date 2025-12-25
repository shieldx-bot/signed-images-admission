package k8s

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetClientset() (*kubernetes.Clientset, error) {
	var kubeconfig *string
	home := homedir.HomeDir()
	if home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		flag.Parse()
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute ")
		flag.Parse()
	}
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Printf("Không tìm thấy file kubeconfig")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Token config không đúng")
	}

	Namespace, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})

}

// Pods, err := clientset.CoreV1().Pods("sre-mart").List(context.TODO(), metav1.ListOptions{})
// 	if err != nil {
// 		fmt.Printf("❌ LỖI LẤY PODS: %v\n", err)
// 		return
// 	}
