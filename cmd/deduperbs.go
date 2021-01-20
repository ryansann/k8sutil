package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"strings"
)

var deduperbsCmd = &cobra.Command{
	Use:   "deduperbs",
	Short: "deduperbs removes duplicate RoleBindings and ClusterRoleBinding resources from a kubernetes cluster",
	Long: "deduperbs deletes dupes from an input file () otherwise it retrieves the list from the kubernetes api server. " +
		"Once it has founds duplicates, it attempts to remove them. Use --dry-run to skip the removal process.",
	Run: runDeduperbs,
}

var (
	dryRun    bool
	inputFile string
	schm      *runtime.Scheme
)

func init() {
	// flags
	deduperbsCmd.PersistentFlags().StringVarP(&inputFile, "input-file", "f", "", "Name of the file containing list of rbs and crbs returned as json from the kubernetes api as a List.")
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

	data, err := readFile(inputFile)
	if err != nil {
		logrus.Fatal("couldn't read file")
	}

	var l corev1.List
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err = decode(data, nil, &l)
	if err != nil {
		logrus.Fatal(err)
	}

	// indexRbSubjRole stores a list of rolebinding uids for each subject/role combination
	indexRbSubjRole := make(map[string][]string)
	// filteredIndexRbSubjRole stores a list of rolebinding uids for each subject/role combination, and additionally only contains dupes
	var filteredIndexRbSubjRole map[string][]string

	logrus.Debugf("list has: %v items", len(l.Items))
	for _, i := range l.Items {
		o, _, err := decode(i.Raw, nil, nil)
		if err != nil {
			logrus.Fatal(err)
		}

		switch o.(type) {
		case *rbacv1.RoleBinding:
			rb := o.(*rbacv1.RoleBinding)
			logrus.Debugf("rb.Name: %v", rb.Name)

			subjRoleKeys := getRbSubjRoleKeys(rb)
			if len(subjRoleKeys) > 1 {
				logrus.Warn("rb: %v has multiple subjects", string(rb.UID))
			}

			filteredKeys := filterRbSubjRoleKeys(subjRoleKeys, []string{"-projectmember", "-projectowner", "-clustermember", "-clusterowner"})
			for _, key := range filteredKeys {
				if existing, ok := indexRbSubjRole[key]; ok {
					existing = append(existing, string(rb.UID))
					indexRbSubjRole[key] = existing
				} else {
					indexRbSubjRole[key] = []string{string(rb.UID)}
				}
			}

			filteredIndexRbSubjRole = filterRbSubjRoleIndex(indexRbSubjRole)
		case *rbacv1.ClusterRoleBinding:
			crb := o.(*rbacv1.ClusterRoleBinding)
			logrus.Debugf("crb.Name: %v", crb.Name)
		}
	}

	ind, err := json.MarshalIndent(filteredIndexRbSubjRole, "", "  ")
	if err != nil {
		logrus.Fatal(err)
	}

	fmt.Printf("%v", string(ind))
}

func getRbSubjRoleKeys(rb *rbacv1.RoleBinding) []string {
	var keys []string
	role := rb.RoleRef.Name
	for _, subj := range rb.Subjects {
		keys = append(keys, strings.Join([]string{subj.Name, role}, "/"))
	}
	return keys
}

func filterRbSubjRoleKeys(keys []string, contains []string) []string {
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

func filterRbSubjRoleIndex(ind map[string][]string) map[string][]string {
	filtered := make(map[string][]string)
	for k, v := range ind {
		if len(v) > 1 {
			filtered[k] = v
		}
	}
	return filtered
}
