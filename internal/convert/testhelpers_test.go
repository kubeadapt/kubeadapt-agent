package convert

import "k8s.io/apimachinery/pkg/api/resource"

func quantityPtr(s string) *resource.Quantity {
	q := resource.MustParse(s)
	return &q
}
