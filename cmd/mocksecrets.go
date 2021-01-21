package cmd

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/ryansann/k8sutil/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var mockSecretsCmd = &cobra.Command{
	Use:   "mocksecrets",
	Short: "mocksecrets creates N secrets in the kubernetes cluster",
	Run:   runMockSecrets,
}

var (
	numSecrets       int
	numSecretWorkers int
	secretSize       int
	seqStart         int
	namespace        string
)

func init() {
	mockSecretsCmd.PersistentFlags().IntVarP(&numSecrets, "num-secrets", "n", 100, "Number of secrets to create")
	mockSecretsCmd.PersistentFlags().IntVarP(&numSecretWorkers, "num-workers", "w", 10, "Number of workers to create secrets")
	mockSecretsCmd.PersistentFlags().IntVarP(&secretSize, "secret-size", "s", 10, "How large the generated secret data is")
	mockSecretsCmd.PersistentFlags().IntVar(&seqStart, "seq-start", 1, "Where to start the sequence for secret naming, e.g. secret-<seq-start>")
	mockSecretsCmd.PersistentFlags().StringVar(&namespace, "ns", "default", "Namespace to create secrets in")
}

func runMockSecrets(cmd *cobra.Command, args []string) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Debug("running mocksecrets command")

	cli, err := k8s.GetClient(kubeConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	// check if namespace exists, create it if it doesn't
	_, err = cli.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) { // create if not found
		ns, err := cli.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
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
	jobs := make(chan int, numSecretWorkers)

	// spawn workers
	var wg sync.WaitGroup
	wg.Add(numSecretWorkers)
	for j := 1; j <= numSecretWorkers; j++ {
		go func(w int) {
			logrus.Debugf("starting worker %v", w)
			workerCli, _ := k8s.GetClient(kubeConfig)
			defer wg.Done()
			for i := range jobs {
				secretNum := seqStart + i
				logrus.Debugf("worker %v creating secret %v", w, secretNum)
				s := genRandomSecret(secretNum)
				_, err := workerCli.CoreV1().Secrets(namespace).Create(&s)
				if err != nil {
					e <- err
				}
			}
		}(j)
	}

	// push work onto jobs channel
	for i := 0; i < numSecrets; i++ {
		jobs <- i
	}
	close(jobs) // exit condition for workers

	wg.Wait() // wait for workers to exit

	secrets, err := batchGetSecrets(cli, "")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("cluster has %v secrets", len(secrets))
}

const charset = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "0123456789"

var seed *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
var mtx sync.Mutex

func randString(length int) string {
	b := make([]byte, length)
	mtx.Lock()
	defer mtx.Unlock()
	for i := range b {
		b[i] = charset[seed.Intn(len(charset))]
	}
	return string(b)
}

// genRandomSecret creates a secret with random data
func genRandomSecret(i int) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("secret-%v", i),
		},
		Data: map[string][]byte{"password": []byte(randString(secretSize))},
	}
}

const (
	secretBatchSize = 100
)

func batchGetSecrets(cli *kubernetes.Clientset, ns string) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	var continueToken string
	for {
		secretsList, err := cli.CoreV1().Secrets(ns).List(metav1.ListOptions{
			Limit:    secretBatchSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, err
		}

		secrets = append(secrets, secretsList.Items...)

		if secretsList.Continue == "" {
			break
		}

		continueToken = secretsList.Continue
	}
	return secrets, nil
}
