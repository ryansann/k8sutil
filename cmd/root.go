package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ryansann/k8sdump/config"
	"github.com/ryansann/k8sdump/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "k8sdump",
	Short: "k8sdump dumps resources from a k8s cluster",
	Run:   run,
}

// run executes the steps required to dump resources
func run(cmd *cobra.Command, args []string) {
	cfgbytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Debugf("config:\n%v\n", string(cfgbytes))

	dumps, err := k8s.GetDumps(cfg)
	if err != nil {
		logrus.Fatal(err)
	}

	dbytes, err := json.MarshalIndent(dumps, "", "  ")
	if err != nil {
		logrus.Fatal(err)
	}

	fmt.Println(string(dbytes))
}

var (
	cfgFile string
	cfg     config.Root
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config-file", "c", "./k8sdump.yaml", "Path to k8sdump config file")
}

func initConfig() {
	logrus.Debugf("using config file: %v", cfgFile)

	viper.SetConfigFile(cfgFile)

	err := viper.ReadInConfig()
	if err != nil {
		logrus.Fatal(err)
	}

	err = viper.Unmarshal(&cfg)
	if err != nil {
		logrus.Fatal(err)
	}
}

// Execute runs the k8sdump root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
