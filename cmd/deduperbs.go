package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/ryansann/k8sutil/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"strings"
)

var deduperbsCmd = &cobra.Command{
	Use:   "deduperbs",
	Short: "deduperbs removes duplicate RoleBindings and ClusterRoleBinding resources from a kubernetes cluster",
	Long: "deduperbs deletes dupes from input files (--input-file-rbs and/or --input-file-crbs) otherwise it retrieves the list from the kubernetes api server. " +
		"Once it has found duplicates, it attempts to remove them. Use --dry-run to skip the removal process.",
	Run: runDeduperbs,
}

var (
	dryRun        bool
	inputFileRbs  string
	inputFileCrbs string
	schm          *runtime.Scheme
)

func init() {
	// flags
	deduperbsCmd.PersistentFlags().StringVar(&inputFileRbs, "input-file-rbs", "", "Name of the file containing list of rolebindings as returned from the kubernetes api as a JSON v1.List")
	deduperbsCmd.PersistentFlags().StringVar(&inputFileCrbs, "input-file-crbs", "", "Name of the file containing list of clusterrolebindings as returned from the kubernetes api as a JSON v1.List")
	deduperbsCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "If set, dupes will not be removed from the kubernetes.")

	// init scheme for decoder
	schm = runtime.NewScheme()
	_ = rbacv1.AddToScheme(schm)
	_ = corev1.AddToScheme(schm)
}

func runDeduperbs(cmd *cobra.Command, args []string) {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Debug("running deduperbs command")

	decode := scheme.Codecs.UniversalDeserializer().Decode
	var rbs, crbs corev1.List

	if inputFileRbs != "" {
		rbsData, err := readFile(inputFileRbs)
		if err != nil {
			logrus.Fatalf("could not read file: %v", inputFileRbs)
		}

		_, _, err = decode(rbsData, nil, &rbs)
		if err != nil {
			logrus.Fatalf("decode error: %v", err)
		}
	}

	if inputFileCrbs != "" {
		crbsData, err := readFile(inputFileCrbs)
		if err != nil {
			logrus.Fatalf("could not read file: %v", inputFileCrbs)
		}

		_, _, err = decode(crbsData, nil, &crbs)
		if err != nil {
			logrus.Fatalf("decode error: %v", err)
		}
	}

	// retrieve from kubernetes if no input files are given
	if inputFileRbs == "" && inputFileCrbs == "" {
		logrus.Debugf("using kubeconfig: %v", kubeConfig)
		rbCli, err := k8s.GetDynamicClient(kubeConfig, schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"})
		if err != nil {
			logrus.Fatalf("error creating dynamic client: %v", err)
		}

		rbsList, err := rbCli.Namespace("").List(metav1.ListOptions{})
		if err != nil {
			logrus.Fatalf("could not retrieve RoleBindings from kubernetes, %v", err)
		}

		rbsData, err := json.Marshal(rbsList)
		if err != nil {
			logrus.Fatalf("marshal error: %v", err)
		}

		_, _, err = decode(rbsData, nil, &rbs)
		if err != nil {
			logrus.Fatalf("decode error: %v", err)
		}

		crbCli, err := k8s.GetDynamicClient(kubeConfig, schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"})
		if err != nil {
			logrus.Fatalf("error creating dynamic client: %v", err)
		}

		crbsList, err := crbCli.Namespace("").List(metav1.ListOptions{})
		if err != nil {
			logrus.Fatalf("could not retrieve ClusterRoleBindings from kubernetes, %v", err)
		}

		crbsData, err := json.Marshal(crbsList)
		if err != nil {
			logrus.Fatalf("marshal error: %v", err)
		}

		_, _, err = decode(crbsData, nil, &crbs)
		if err != nil {
			logrus.Fatalf("decode error: %v", err)
		}
	}

	rbDupes, err := findDupes(rbs)
	if err != nil {
		logrus.Fatalf("could not find rb dupes: %v", err)
	}

	var foundDupeRbs, foundDupeCrbs bool
	out := make(map[string]interface{})

	if len(rbDupes) > 0 {
		foundDupeRbs = true
		out["rolebindings"] = rbDupes
	} else {
		logrus.Debug("no dupe rbs found")
	}

	crbDupes, err := findDupes(crbs)
	if err != nil {
		logrus.Fatalf("could not find crb dupes: %v", err)
	}

	if len(crbDupes) > 0 {
		foundDupeCrbs = true
		out["clusterrolebindings"] = crbDupes
	} else {
		logrus.Debug("no dupe crbs found")
	}

	outBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		logrus.Fatal(err)
	}

	if foundDupeRbs || foundDupeCrbs {
		fmt.Printf("%v", string(outBytes))
	}
}

var (
	filters = []string{"-projectmember", "-projectowner", "-clustermember", "-clusterowner"}
)

func findDupes(l corev1.List) (map[string][]string, error) {
	// index stores a list of RoleBinding/ClusterRoleBinding uids for each subject/role combination
	index := make(map[string][]string)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	logrus.Debugf("list has: %v items", len(l.Items))
	for _, i := range l.Items {
		o, _, err := decode(i.Raw, nil, nil)
		if err != nil {
			logrus.Error(err)
			continue
		}

		var uid string
		var filteredKeys []string

		switch o.(type) {
		case *rbacv1.RoleBinding:
			rb := o.(*rbacv1.RoleBinding)
			logrus.Debugf("rb.Name: %v", rb.Name)

			subjRoleKeys := getRbSubjRoleNsKeys(rb)
			if len(subjRoleKeys) > 1 {
				logrus.Debugf("rb: %v has multiple subjects", string(rb.UID))
			}

			filteredKeys = filterSubjRoleKeys(subjRoleKeys, filters)
			uid = string(rb.UID)
		case *rbacv1.ClusterRoleBinding:
			crb := o.(*rbacv1.ClusterRoleBinding)
			logrus.Debugf("crb.Name: %v", crb.Name)

			subjRoleKeys := getCrbSubjRoleKeys(crb)
			if len(subjRoleKeys) > 1 {
				logrus.Debugf("crb: %v has multiple subjects", string(crb.UID))
			}

			filteredKeys = filterSubjRoleKeys(subjRoleKeys, filters)
			uid = string(crb.UID)
		default:
			return nil, fmt.Errorf("unexpected type in list")
		}

		for _, key := range filteredKeys {
			if existing, ok := index[key]; ok {
				existing = append(existing, uid)
				index[key] = existing
			} else {
				index[key] = []string{uid}
			}
		}
	}

	return findSubjRoleDupes(index), nil
}

func getRbSubjRoleNsKeys(rb *rbacv1.RoleBinding) []string {
	var keys []string
	role := rb.RoleRef.Name
	for _, subj := range rb.Subjects {
		keys = append(keys, strings.Join([]string{subj.Name, role, rb.Namespace}, "/"))
	}
	return keys
}

func filterSubjRoleKeys(keys []string, contains []string) []string {
	var filtered []string
	for _, k := range keys {
		hasSubstr := false
		for _, substr := range contains {
			if strings.Contains(k, substr) {
				hasSubstr = true
				break
			}
		}
		if hasSubstr {
			filtered = append(filtered, k)
		}
	}
	return filtered
}

func findSubjRoleDupes(ind map[string][]string) map[string][]string {
	filtered := make(map[string][]string)
	for k, v := range ind {
		if len(v) > 1 {
			filtered[k] = v
		}
	}
	return filtered
}

func getCrbSubjRoleKeys(crb *rbacv1.ClusterRoleBinding) []string {
	var keys []string
	role := crb.RoleRef.Name
	for _, subj := range crb.Subjects {
		keys = append(keys, strings.Join([]string{subj.Name, role}, "/"))
	}
	return keys
}
