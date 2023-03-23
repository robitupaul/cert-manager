package contour

import (
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	httpProxyGvr = schema.GroupVersionResource{Group: "projectcontour.io", Version: "v1", Resource: "httpproxies"}
	httpProxyGvk = schema.GroupVersionKind{Group: "projectcontour.io", Version: "v1", Kind: "HTTPProxy"}
)

func HTTPProxyGvr() schema.GroupVersionResource {
	return httpProxyGvr
}

type HTTPProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   contourv1.HTTPProxySpec   `json:"spec,omitempty"`
	Status contourv1.HTTPProxyStatus `json:"status"`
}

// HTTPProxyList is a collection of HTTPProxy.
type HTTPProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPProxy `json:"items"`
}

func (httpProxy *HTTPProxy) ToUnstructured() (*unstructured.Unstructured, error) {
	httpProxy.TypeMeta.SetGroupVersionKind(httpProxyGvk)
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(httpProxy)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}

func HTTPProxyFromUnstructured(unstr *unstructured.Unstructured) (*HTTPProxy, error) {
	var httpProxy HTTPProxy
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy)
	if err != nil {
		return nil, err
	}
	return &httpProxy, nil
}
