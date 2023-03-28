package http

import (
	"context"
	"reflect"
	"testing"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	"github.com/cert-manager/cert-manager/pkg/issuer/acme/http/contour"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestEnsureContour(t *testing.T) {
	const svcName = "fakeservice"

	const httpProxySpecKey = "httpproxyspeckey"

	httpProxyGvr := contour.HTTPProxyGvr()

	testChallenge := cmacme.Challenge{
		Spec: cmacme.ChallengeSpec{
			DNSName: "example.com",
			Solver: cmacme.ACMEChallengeSolver{
				HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
					HTTPProxy: &cmacme.ACMEChallengeSolverHTTP01HTTPProxy{},
				},
			},
		},
	}

	tests := map[string]solverFixture{
		"should create HTTPProxy": {
			Challenge: &testChallenge,
			CheckFn: func(t *testing.T, s *solverFixture, args ...interface{}) {
				hps, err := s.Solver.httpProxyLister.List(labels.NewSelector())
				if err != nil {
					t.Errorf("error listing HTTPProxy: %v", err)
				}
				if len(hps) != 1 {
					t.Errorf("expected one HTTPProxy to be created, but %d HTTPProxies were found", len(hps))
				}
			},
		},
		"should not modify correct HTTPProxy": {
			Challenge: &testChallenge,
			PreFn: func(t *testing.T, s *solverFixture) {
				httpProxySpec := createHTTPProxySpec(&testChallenge, svcName)
				s.testResources[httpProxySpecKey] = httpProxySpec
				httpProxy := contour.HTTPProxy{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName:    "test-httpproxy-",
						Namespace:       testChallenge.Namespace,
						Labels:          podLabels(&testChallenge),
						OwnerReferences: []metav1.OwnerReference{},
					},
					Spec: *httpProxySpec,
				}
				unstr, err := httpProxy.ToUnstructured()
				if err != nil {
					t.Errorf("error converting to unstructured: %v", err)
				}
				_, err = s.FakeDynamicClient().Resource(httpProxyGvr).Namespace(testChallenge.Namespace).Create(context.Background(), unstr, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("error preparing test: %v", err)
				}
			},
			CheckFn: func(t *testing.T, s *solverFixture, args ...interface{}) {
				hps, err := s.Solver.httpProxyLister.List(labels.NewSelector())
				if err != nil {
					t.Errorf("error listing HTTPProxy: %v", err)
					return
				}
				if len(hps) != 1 {
					t.Errorf("expected one HTTPProxy to be created by %d HTTPProxies were found", len(hps))
					return
				}
				newHTTPProxy, err := contour.HTTPProxyFromUnstructured(hps[0])
				if err != nil {
					t.Errorf("could not decode retrieved HTTPProxy: %v", err)
					return
				}
				oldHTTPProxySpec := s.testResources[httpProxySpecKey]
				newHTTPProxySpec := &newHTTPProxy.Spec
				if reflect.TypeOf(oldHTTPProxySpec) != reflect.TypeOf(newHTTPProxySpec) {
					t.Errorf("types should not be equal (error in test)")
				}
				if !reflect.DeepEqual(oldHTTPProxySpec, newHTTPProxySpec) {
					t.Errorf("did not expect correct HTTPProxy to be modified")
				}
			},
		},
		"should fix existing HTTPProxy": {
			Challenge: &testChallenge,
			PreFn: func(t *testing.T, s *solverFixture) {
				httpProxySpec := createHTTPProxySpec(&testChallenge, svcName+"needs-fixing")
				s.testResources[httpProxySpecKey] = httpProxySpec
				httpProxy := contour.HTTPProxy{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName:    "test-gateway-",
						Namespace:       testChallenge.Namespace,
						Labels:          podLabels(&testChallenge),
						OwnerReferences: []metav1.OwnerReference{},
					},
					Spec: *httpProxySpec,
				}
				unstr, err := httpProxy.ToUnstructured()
				if err != nil {
					t.Errorf("error converting to unstructured: %v", err)
				}
				_, err = s.FakeDynamicClient().Resource(httpProxyGvr).Namespace(testChallenge.Namespace).Create(context.Background(), unstr, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("error preparing test: %v", err)
				}
			},
			CheckFn: func(t *testing.T, s *solverFixture, args ...interface{}) {
				hps, err := s.Solver.httpProxyLister.List(labels.NewSelector())
				if err != nil {
					t.Errorf("error listing HTTPProxies: %v", err)
					return
				}
				if len(hps) != 1 {
					t.Errorf("expected one HTTPProxy to be created, but %d were found", len(hps))
					return
				}
				newHTTPProxy, err := contour.HTTPProxyFromUnstructured(hps[0])
				if err != nil {
					t.Errorf("could not decode retrived HTTPProxy: %v", err)
					return
				}
				oldHTTPProxySpec := s.testResources[httpProxySpecKey]
				newHTTPPRoxySpec := &newHTTPProxy.Spec
				if reflect.TypeOf(oldHTTPProxySpec) != reflect.TypeOf(newHTTPPRoxySpec) {
					t.Errorf("types should be equal (error in test)")
				}
				if reflect.DeepEqual(oldHTTPProxySpec, newHTTPPRoxySpec) {
					t.Errorf("expected existing HTTPProxy spec to be fixed")
				}
				if newHTTPPRoxySpec.Routes[0].Services[0].Name != svcName {
					t.Errorf("expected HTTPProxy destination service to be fixed")
				}
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.Setup(t)
			resp, err := test.Solver.ensureHTTPProxy(context.TODO(), test.Challenge, svcName)
			if err != nil && !test.Err {
				t.Errorf("Expected function to not error, but got: %v", err)
			}
			if err != nil && test.Err {
				t.Errorf("Expected function to get an error, but got: %v", err)
			}
			test.Finish(t, resp, err)
		})
	}
}
