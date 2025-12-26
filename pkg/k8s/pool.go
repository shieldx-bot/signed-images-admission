package k8s

import (
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
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute ")
	}
	if !flag.Parsed() {
		flag.Parse()
	}
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("không tìm thấy file kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("token/config không đúng: %w", err)
	}

	return clientset, nil
}

// Pods, err := clientset.CoreV1().Pods("sre-mart").List(context.TODO(), metav1.ListOptions{})
// 	if err != nil {
// 		fmt.Printf("❌ LỖI LẤY PODS: %v\n", err)
// 		return
// 	}
