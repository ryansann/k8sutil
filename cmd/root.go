package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "k8sutil",
	Short: "k8sutil performs helper operations on a kubernetes cluster",
	Run:   run,
}

var (
	kubeConfig string
	debug      bool
)

func init() {
	rootCmd.AddCommand(
		dumpCmd,
		mockSecretsCmd,
		pushImagesCmd,
		deduperbsCmd,
	)
	rootCmd.PersistentFlags().StringVarP(&kubeConfig, "kube-config", "c", "~/.kube/config", "Kubeconfig file for cluster")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
}

// run executes the steps required to dump resources
func run(cmd *cobra.Command, args []string) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Debugf("running root command")
	logrus.Debugf("using kubeconfig: %s", kubeConfig)
}

// Execute runs the k8sutil root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
