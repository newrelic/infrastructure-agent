// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metadata

import (
	"os"
	"strings"
)

func GatherK8sMetadata() (meta map[string]interface{}) {
	meta = make(map[string]interface{})

	field2Env := map[string]string{
		"clusterName":    "NEW_RELIC_METADATA_KUBERNETES_CLUSTER_NAME",
		"nodeName":       "NEW_RELIC_METADATA_KUBERNETES_NODE_NAME",
		"namespaceName":  "NEW_RELIC_METADATA_KUBERNETES_NAMESPACE_NAME",
		"deploymentName": "NEW_RELIC_METADATA_KUBERNETES_DEPLOYMENT_NAME",
		"podName":        "NEW_RELIC_METADATA_KUBERNETES_POD_NAME",
		"containerName":  "NEW_RELIC_METADATA_KUBERNETES_CONTAINER_NAME",
		"containerImage": "NEW_RELIC_METADATA_KUBERNETES_CONTAINER_IMAGE_NAME",
	}
	for metaName, envName := range field2Env {
		if envVal := os.Getenv(envName); len(envVal) > 0 {
			meta[metaName] = envVal
		}
	}

	labels := os.Getenv("NEW_RELIC_METADATA_KUBERNETES_LABELS")
	for _, label := range strings.Split(labels, ",") {
		splits := strings.Split(label, "=")
		if len(splits) == 2 {
			meta["label."+splits[0]] = splits[1]
		}
	}

	return
}
