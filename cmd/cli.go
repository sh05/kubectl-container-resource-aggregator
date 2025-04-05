package main

import (
	"fmt"
	"io"
	"os"

	"github.com/sh05/kubectl-container-resource-aggregator/resource"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		exitWithError("入力読み込みエラー", err)
	}

	obj, podSpec, err := decodeManifest(data)
	if err != nil {
		exitWithError("マニフェスト解析エラー", err)
	}

	summary, err := resource.Aggregate(podSpec)
	if err != nil {
		exitWithError("リソース集計エラー", err)
	}

	printResult(obj, summary)
}

func decodeManifest(data []byte) (*unstructured.Unstructured, map[string]interface{}, error) {
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := dec.Decode(data, nil, obj)
	if err != nil {
		return nil, nil, err
	}

	podSpec, err := extractPodSpec(obj)
	if err != nil {
		return nil, nil, err
	}

	return obj, podSpec, nil
}

func extractPodSpec(obj *unstructured.Unstructured) (map[string]interface{}, error) {
	switch obj.GetKind() {
	case "Deployment", "StatefulSet", "DaemonSet":
		return unstructured.NestedMap(obj.Object, "spec", "template", "spec")
	case "Pod":
		return unstructured.NestedMap(obj.Object, "spec")
	default:
		return nil, fmt.Errorf("未対応のリソースタイプ: %s", obj.GetKind())
	}
}

func printResult(obj *unstructured.Unstructured, s *resource.Summary) {
	fmt.Printf("解析結果 (%s/%s):\n", obj.GetKind(), obj.GetName())
	fmt.Println(s.Format())
}

func exitWithError(context string, err error) {
	fmt.Fprintf(os.Stderr, "エラー: %s - %v\n", context, err)
	os.Exit(1)
}
