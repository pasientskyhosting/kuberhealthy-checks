package main

import (
	"context"
	"os"
	"path/filepath"

	checkclient "github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/Comcast/kuberhealthy/v2/pkg/kubeClient"

	// required for oidc kubectl testing
	log "github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// KubeConfigFile is a variable containing file path of Kubernetes config files
var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")
var namespace string
var skipDurationEnv string

func init() {
	checkclient.Debug = true
}

type Options struct {
	client kubernetes.Interface
}

func main() {
	var err error
	o := Options{}
	o.client, err = kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	// get our list of duplicate ingresses, if there are any errors, report failures to Kuberhealthy servers.
	duplicates, err := o.findDuplicateIngress()
	if err != nil {
		err = checkclient.ReportFailure([]string{err.Error()})
		if err != nil {
			log.Println("Error", err)
			os.Exit(1)
		}
		return
	}
	// report our list of duplicate ingresses to Kuberhealthy servers.
	if len(duplicates) >= 1 {
		log.Infoln("Number of duplicate ingresses found: ", len(duplicates))
		err = checkclient.ReportFailure(duplicates)
		if err != nil {
			log.Println("Error reporting failures to Kuberhealthy servers", err)
			os.Exit(1)
		}
		return
	}
	// report success to Kuberhealthy servers if there were no duplicate ingresses in our list.
	err = checkclient.ReportSuccess()
	log.Infoln("Reporting Success, no duplicate ingresses found.")
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers", err)
		os.Exit(1)
	}
}

// finds duplicate ingresses
func (o Options) findDuplicateIngress() ([]string, error) {

	var hosts []string
	var duplicates []string

	namespace = os.Getenv("TARGET_NAMESPACE")
	if namespace == "" {
		log.Println("looking for ingresses across all namespaces, this requires a cluster role")
		// it is the same value but we are being explicit that we are listing ingresses in all namespaces
		namespace = v1.NamespaceAll
	} else {
		log.Printf("looking for ingresses in namespace %s", namespace)
	}

	ingressList, err := o.client.ExtensionsV1beta1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return duplicates, err
	}

	// start iteration over ingresses
	for _, ingress := range ingressList.Items {
		for _, rule := range ingress.Spec.Rules {
			if contains(hosts, rule.Host) {
				duplicates = append(duplicates, rule.Host)
			} else {
				hosts = append(hosts, rule.Host)
			}
		}
	}
	return duplicates, nil

}

// checks if item exists in slice
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
