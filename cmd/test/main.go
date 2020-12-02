package main

import (
	"context"

	"github.com/ryansann/k8sutil/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kubeConfig = "../../../resources/.kube/config"
	ns         = "default"
	secretName = "secret-111"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	logrus.Debug("running mocksecrets command")

	cli, err := k8s.GetClient(kubeConfig)
	if err != nil {
		logrus.Fatal(err)
	}

	s, err := cli.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		logrus.Fatalf("get error: %v", err)
	}

	err = cli.CoreV1().Secrets(ns).Delete(context.Background(), secretName, metav1.DeleteOptions{})
	if err != nil {
		logrus.Fatalf("delete error: %v", err)
	}

	s, err = cli.CoreV1().Secrets(ns).Update(context.Background(), s, metav1.UpdateOptions{})
	if err != nil {
		e := err.(errors.APIStatus)
		logrus.Debug(e.Status())
		if errors.IsConflict(err) {
			logrus.Error("conflict")
		}
		s, err = cli.CoreV1().Secrets(ns).Get(context.Background(), secretName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Error("not found")
			}
		}
		logrus.Fatalf("update error: %v", err)
	}
}
