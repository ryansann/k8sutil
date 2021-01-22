package cmd

import (
	"encoding/json"
	"fmt"
	"k8s.io/client-go/kubernetes"

	"github.com/ryansann/k8sutil/k8s"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	var cli *kubernetes.Clientset
	var err error
	if kubeConfig != "" {
		logrus.Debugf("using kubeconfig: %v", kubeConfig)
		cli, err = k8s.GetClient(kubeConfig)
		if err != nil {
			logrus.Fatalf("error creating k8s client: %v", err)
		}
	} else {
		dryRun = true
		logrus.Debugf("no kubeconfig provided, dupes will not be removed from api")
	}

	var rbInd, crbInd map[string][]string
	if inputFileRbs == "" && inputFileCrbs == "" {
		rbInd, crbInd = findDupesFromK8s(cli)
	} else {
		rbInd, crbInd = findDupesFromFiles()
	}

	var foundDupeRbs, foundDupeCrbs bool
	out := make(map[string]interface{})

	if len(rbInd) > 0 {
		foundDupeRbs = true
		out["rolebindings"] = rbInd
	} else {
		logrus.Debug("no dupe rbs found")
	}

	if len(crbInd) > 0 {
		foundDupeCrbs = true
		out["clusterrolebindings"] = crbInd
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

	if !dryRun {
		err = removeDupeRbs(cli, rbInd)
		if err != nil {
			logrus.Fatal("could not remove dupe rbs: %v", err)
		}

		err = removeDupeCrbs(cli, crbInd)
		if err != nil {
			logrus.Fatal("could not remove dupe crbs: %v", err)
		}
	}
}

func findDupesFromFiles() (map[string][]string, map[string][]string) {
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

	rbDupes, err := findDupes(rbs)
	if err != nil {
		logrus.Fatalf("could not find rb dupes: %v", err)
	}

	crbDupes, err := findDupes(crbs)
	if err != nil {
		logrus.Fatalf("could not find crb dupes: %v", err)
	}

	return rbDupes, crbDupes
}

func findDupesFromK8s(cli *kubernetes.Clientset) (map[string][]string, map[string][]string) {
	rbsList, err := cli.RbacV1().RoleBindings("").List(metav1.ListOptions{})
	if err != nil {
		logrus.Fatalf("could not retrieve RoleBindings from kubernetes, %v", err)
	}

	// index stores a list of RoleBinding/ClusterRoleBinding uids for each subject/role combination
	rbIndex := make(map[string][]string)
	for _, rb := range rbsList.Items {
		var filteredKeys []string
		id := strings.Join([]string{rb.Namespace, rb.Name}, "/")

		subjRoleKeys := getRbSubjRoleNsKeys(&rb)
		if len(subjRoleKeys) > 1 {
			logrus.Debugf("rb: %v has multiple subjects", id)
		}

		filteredKeys = filterKeys(subjRoleKeys, filters)

		for _, key := range filteredKeys {
			if existing, ok := rbIndex[key]; ok {
				existing = append(existing, id)
				rbIndex[key] = existing
			} else {
				rbIndex[key] = []string{id}
			}
		}
	}

	crbsList, err := cli.RbacV1().ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		logrus.Fatalf("could not retrieve ClusterRoleBindings from kubernetes, %v", err)
	}

	// index stores a list of RoleBinding/ClusterRoleBinding uids for each subject/role combination
	crbIndex := make(map[string][]string)
	for _, crb := range crbsList.Items {
		var filteredKeys []string
		id := strings.Join([]string{crb.Namespace, crb.Name}, "/")

		subjRoleKeys := getCrbSubjRoleKeys(&crb)
		if len(subjRoleKeys) > 1 {
			logrus.Debugf("rb: %v has multiple subjects", id)
		}

		filteredKeys = filterKeys(subjRoleKeys, filters)

		for _, key := range filteredKeys {
			if existing, ok := crbIndex[key]; ok {
				existing = append(existing, id)
				crbIndex[key] = existing
			} else {
				crbIndex[key] = []string{id}
			}
		}
	}

	return filterDupes(rbIndex), filterDupes(crbIndex)
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

		var id string
		var filteredKeys []string

		switch o.(type) {
		case *rbacv1.RoleBinding:
			rb := o.(*rbacv1.RoleBinding)
			logrus.Debugf("rb.Name: %v", rb.Name)

			subjRoleKeys := getRbSubjRoleNsKeys(rb)
			if len(subjRoleKeys) > 1 {
				logrus.Debugf("rb: %v has multiple subjects", string(rb.UID))
			}

			filteredKeys = filterKeys(subjRoleKeys, filters)
			id = strings.Join([]string{rb.Namespace, rb.Name}, "/")
		case *rbacv1.ClusterRoleBinding:
			crb := o.(*rbacv1.ClusterRoleBinding)
			logrus.Debugf("crb.Name: %v", crb.Name)

			subjRoleKeys := getCrbSubjRoleKeys(crb)
			if len(subjRoleKeys) > 1 {
				logrus.Debugf("crb: %v has multiple subjects", string(crb.UID))
			}

			filteredKeys = filterKeys(subjRoleKeys, filters)
			id = strings.Join([]string{crb.Namespace, crb.Name}, "/")
		default:
			return nil, fmt.Errorf("unexpected type in list")
		}

		for _, key := range filteredKeys {
			if existing, ok := index[key]; ok {
				existing = append(existing, id)
				index[key] = existing
			} else {
				index[key] = []string{id}
			}
		}
	}

	return filterDupes(index), nil
}

func getRbSubjRoleNsKeys(rb *rbacv1.RoleBinding) []string {
	var keys []string
	role := rb.RoleRef.Name
	for _, subj := range rb.Subjects {
		keys = append(keys, strings.Join([]string{subj.Name, role, rb.Namespace}, "/"))
	}
	return keys
}

func filterKeys(keys []string, contains []string) []string {
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

func filterDupes(ind map[string][]string) map[string][]string {
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

func removeDupeRbs(cli *kubernetes.Clientset, ind map[string][]string) error {
	for k, v := range ind {
		logrus.Debugf("processing dupes for %v", k)
		if len(v) > 1 { // there are dupes
			for _, id := range v[1:] { // skip first element
				cmps := strings.Split(id, "/")
				ns, name := cmps[0], cmps[1]

				logrus.Debugf("removing rb: %s/%s", ns, name)
				err := cli.RbacV1().RoleBindings(ns).Delete(name, &metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func removeDupeCrbs(cli *kubernetes.Clientset, ind map[string][]string) error {
	for k, v := range ind {
		logrus.Debugf("processing dupes for %v", k)
		if len(v) > 1 { // there are dupes
			for _, id := range v[1:] { // skip first element
				cmps := strings.Split(id, "/")
				name := cmps[1]

				logrus.Debugf("removing crb: %s", name)
				err := cli.RbacV1().ClusterRoleBindings().Delete(name, &metav1.DeleteOptions{})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
