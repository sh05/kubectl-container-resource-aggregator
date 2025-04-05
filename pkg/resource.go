package resource

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Summary struct {
	TotalRequests     map[string]resource.Quantity
	TotalLimits       map[string]resource.Quantity
	InitMaxRequests   map[string]resource.Quantity
	InitMaxLimits     map[string]resource.Quantity
	EffectiveRequests map[string]resource.Quantity
	EffectiveLimits   map[string]resource.Quantity
}

func NewSummary() *Summary {
	return &Summary{
		TotalRequests:     make(map[string]resource.Quantity),
		TotalLimits:       make(map[string]resource.Quantity),
		InitMaxRequests:   make(map[string]resource.Quantity),
		InitMaxLimits:     make(map[string]resource.Quantity),
		EffectiveRequests: make(map[string]resource.Quantity),
		EffectiveLimits:   make(map[string]resource.Quantity),
	}
}

func Aggregate(podSpec map[string]interface{}) (*Summary, error) {
	s := NewSummary()

	if err := processContainers(podSpec, "initContainers", s, true); err != nil {
		return nil, err
	}

	if err := processContainers(podSpec, "containers", s, false); err != nil {
		return nil, err
	}

	calculateEffective(s)
	return s, nil
}

func processContainers(podSpec map[string]interface{}, field string, s *Summary, isInit bool) error {
	containers, found, err := unstructured.NestedSlice(podSpec, field)
	if err != nil {
		return fmt.Errorf("%s取得エラー: %w", field, err)
	}
	if !found {
		return nil
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		res, _ := container["resources"].(map[string]interface{})
		processResource(res, "requests", s, isInit, updateMax, sumValues)
		processResource(res, "limits", s, isInit, updateMax, sumValues)
	}
	return nil
}

func processResource(res map[string]interface{}, rt string, s *Summary, isInit bool,
	initFn func(dest, src map[string]resource.Quantity),
	mainFn func(dest, src map[string]resource.Quantity),
) {
	resources, _ := res[rt].(map[string]interface{})
	qtyMap := make(map[string]resource.Quantity)

	for k, v := range resources {
		qty, err := resource.ParseQuantity(fmt.Sprintf("%v", v))
		if err == nil {
			qtyMap[k] = qty
		}
	}

	if isInit {
		initFn(getTargetMap(rt, s, true), qtyMap)
	} else {
		mainFn(getTargetMap(rt, s, false), qtyMap)
	}
}

func getTargetMap(rt string, s *Summary, isInit bool) map[string]resource.Quantity {
	if isInit {
		if rt == "requests" {
			return s.InitMaxRequests
		}
		return s.InitMaxLimits
	}
	if rt == "requests" {
		return s.TotalRequests
	}
	return s.TotalLimits
}

func updateMax(dest, src map[string]resource.Quantity) {
	for k, v := range src {
		if current, exists := dest[k]; !exists || v.Cmp(current) > 0 {
			dest[k] = v
		}
	}
}

func sumValues(dest, src map[string]resource.Quantity) {
	for k, v := range src {
		if current, exists := dest[k]; exists {
			current.Add(v)
			dest[k] = current
		} else {
			dest[k] = v
		}
	}
}

func calculateEffective(s *Summary) {
	for res, qty := range s.InitMaxRequests {
		if total, exists := s.TotalRequests[res]; exists && total.Cmp(qty) > 0 {
			s.EffectiveRequests[res] = total
		} else {
			s.EffectiveRequests[res] = qty
		}
	}

	for res, qty := range s.InitMaxLimits {
		if total, exists := s.TotalLimits[res]; exists && total.Cmp(qty) > 0 {
			s.EffectiveLimits[res] = total
		} else {
			s.EffectiveLimits[res] = qty
		}
	}
}

func (s *Summary) Format() string {
	var result string
	resources := make([]string, 0, len(s.EffectiveRequests))

	for k := range s.EffectiveRequests {
		resources = append(resources, k)
	}
	sort.Strings(resources)

	for _, res := range resources {
		result += fmt.Sprintf("%s:\n", res)
		result += fmt.Sprintf("  InitMax Requests: %s\n", s.InitMaxRequests[res])
		result += fmt.Sprintf("  Total Requests:   %s\n", s.TotalRequests[res])
		result += fmt.Sprintf("  Effective Request: %s\n", s.EffectiveRequests[res])

		if lim, exists := s.EffectiveLimits[res]; exists {
			result += fmt.Sprintf("  InitMax Limits:   %s\n", s.InitMaxLimits[res])
			result += fmt.Sprintf("  Total Limits:     %s\n", s.TotalLimits[res])
			result += fmt.Sprintf("  Effective Limit:  %s\n", lim)
		}
		result += "\n"
	}
	return result
}
