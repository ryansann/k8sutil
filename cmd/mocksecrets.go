package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/ryansann/k8sutil/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

var mockSecretsCmd = &cobra.Command{
	Use:   "mocksecrets",
	Short: "mocksecrets creates N secrets in the kubernetes cluster",
	Run:   runMockSecrets,
}

var (
	numSecrets int
	numWorkers int
	namespace  string
)

func init() {
	mockSecretsCmd.PersistentFlags().IntVarP(&numSecrets, "num-secrets", "n", 100, "Number of secrets to create")
	mockSecretsCmd.PersistentFlags().IntVarP(&numWorkers, "num-workers", "w", 10, "Number of workers to create secrets")
	mockSecretsCmd.PersistentFlags().StringVar(&namespace, "ns", "default", "Namespace to create secrets in")
}

func runMockSecrets(cmd *cobra.Command, args []string) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Debugf("running mocksecrets command")

	cli, err := k8s.GetClient(kubeConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	// check if namespace exists, create it if it doesn't
	_, err = cli.CoreV1().Namespaces().Get(context.Background(), namespace, v1.GetOptions{})
	if errors.IsNotFound(err) { // create if not found
		ns, err := cli.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: namespace}}, v1.CreateOptions{})
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Debugf("created namespace: %s", ns.Name)
	} else if err != nil {
		logrus.Fatal(err)
	}

	// error logging
	e := make(chan error, 1)
	defer close(e)
	go func() {
		for err := range e {
			logrus.Error(err)
		}
	}()

	// buffered channel for work
	jobs := make(chan int, numWorkers)

	// spawn workers
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for j := 1; j <= numWorkers; j++ {
		go func() {
			logrus.Debugf("starting worker %v", j)
			defer wg.Done()
			for i := range jobs {
				logrus.Debugf("creating secret: %v", i)
				s := genRandomSecret(i)
				_, err := cli.CoreV1().Secrets(namespace).Create(context.Background(), &s, v1.CreateOptions{})
				if err != nil {
					e <- err
				}
			}
		}()
	}

	// push work onto jobs channel
	for i := 1; i <= numSecrets; i++ {
		jobs <- i
	}
	close(jobs) // exit condition for workers

	wg.Wait() // wait for workers to exit
}

// genRandomSecret creates a secret with random data
func genRandomSecret(i int) corev1.Secret {
	randData := sha256.Sum256([]byte(fmt.Sprintf("%v", time.Now().UnixNano())))
	return corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name: fmt.Sprintf("secret-%v", i),
		},
		Data: map[string][]byte{"password": randData[:]},
	}
}
