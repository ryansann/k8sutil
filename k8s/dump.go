package k8s

import (
	"encoding/json"
	"strings"

	"github.com/ryansann/k8sutil/config"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetDumps returns a map of resource dumps that satisfy GVRs and filters, map is keyed by resource
func GetDumps(kubeConfig string, cfg config.DumpCommand) (map[string]interface{}, error) {
	dumps := make(map[string]interface{}, 0)

	for _, dump := range cfg.Dumps {
		cli, err := GetDynamicClient(kubeConfig, dump.GVR)
		if err != nil {
			return nil, err
		}

		l, err := cli.Namespace(dump.Namespace).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		dumps[dump.GVR.Resource] = filterList(l, dump.Filters)
	}

	return dumps, nil
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
