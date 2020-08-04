package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ryansann/k8sdump/config"
	"github.com/ryansann/k8sdump/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	dumps := make(map[string]interface{}, 0)

	for _, dump := range cfg.Dumps {
		cli, err := k8s.GetClient(cfg.KubeConfig, dump.GVR)
		if err != nil {
			logrus.Fatal(err)
		}

		l, err := cli.Namespace(dump.Namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Fatal(err)
		}

		dumps[dump.GVR.Resource] = filterList(l, dump.Filters)
	}

	dbytes, err := json.MarshalIndent(dumps, "", "  ")
	if err != nil {
		logrus.Fatal(err)
	}

	fmt.Println(string(dbytes))
}

// filterList applies filters to a list of resources by json path
func filterList(l *unstructured.UnstructuredList, filter config.Filter) []unstructured.Unstructured {
	var filtered []unstructured.Unstructured
	for _, elt := range l.Items {
		eraw, err := json.Marshal(elt.Object)
		if err != nil {
			logrus.Fatal(err)
		}

		raw := string(eraw)

		// apply and filters, all must be satisfied in order to keep element in filtered list
		andsSatisfied := true
		if len(filter.Ands) > 0 {
			for _, f := range filter.Ands {
				result := gjson.Get(raw, f.Key)
				if !result.Exists() || !strings.EqualFold(result.String(), f.Value) {
					andsSatisfied = false
					break
				}
			}
		}

		// apply or filters, one must be satisfied in order to keep element in filtered list
		orsSatisfied := true
		if len(filter.Ors) > 0 {
			var match bool
			for _, f := range filter.Ors {
				result := gjson.Get(raw, f.Key)
				if result.Exists() && strings.EqualFold(result.String(), f.Value) {
					match = true
				}
			}

			if !match {
				orsSatisfied = false
			}
		}

		if andsSatisfied && orsSatisfied {
			filtered = append(filtered, elt)
		}
	}

	return filtered
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
