package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ryansann/k8sutil/config"
	"github.com/ryansann/k8sutil/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "dump dumps a list of resources from the kubernetes cluster and filters them",
	Run:   runDump,
}

var (
	dumpConfigFile string
	cfg            config.DumpCommand
)

func init() {
	dumpCmd.PersistentFlags().StringVar(&dumpConfigFile, "config", "./dump.yaml", "Path to dump config file")
}

func initConfig() {
	logrus.Debugf("using config file: %v", dumpConfigFile)

	viper.SetConfigFile(dumpConfigFile)

	err := viper.ReadInConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	err = viper.Unmarshal(&cfg)
	if err != nil {
		logrus.Fatal(err)
	}
}

func runDump(cmd *cobra.Command, args []string) {
	cobra.OnInitialize(initConfig)

	dumps, err := k8s.GetDumps(kubeConfig, cfg)
	if err != nil {
		logrus.Fatal(err)
	}

	dbytes, err := json.MarshalIndent(dumps, "", "  ")
	if err != nil {
		logrus.Fatal(err)
	}

	fmt.Println(string(dbytes))
}
