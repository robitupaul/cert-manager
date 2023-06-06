package http

import (
	"context"
	"fmt"
	"reflect"

	cmacme "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	"github.com/cert-manager/cert-manager/pkg/issuer/acme/http/contour"
	logf "github.com/cert-manager/cert-manager/pkg/logs"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (s *Solver) ensureHTTPProxy(ctx context.Context, ch *cmacme.Challenge, svcName string) (*contour.HTTPProxy, error) {
	log := logf.FromContext(ctx).WithName("ensureHTTPProxy")

	httpProxy, err := s.getHTTPProxy(ctx, ch)
	if err != nil {
		return nil, err
	}

	if httpProxy == nil {
		log.Info("creating HTTPProxy")
		httpProxy, err := s.createHTTPProxy(ctx, ch, svcName)
		if err != nil {
			return nil, err
		}

		log.Info("created HTTPProxy successfully")
		return httpProxy, nil
	}

	log.Info("found HTTPProxy")

	httpProxy, err = s.checkAndUpdateHTTPProxy(ctx, ch, svcName, httpProxy)
	if err != nil {
		return nil, err
	}
	return httpProxy, nil
}

func (s *Solver) getHTTPProxy(ctx context.Context, ch *cmacme.Challenge) (*contour.HTTPProxy, error) {
	log := logf.FromContext(ctx, "getHTTPProxy")

	selector := labels.Set(podLabels(ch)).AsSelector()
	hpList, err := s.httpProxyLister.Namespace(ch.Namespace).List(selector)
	if err != nil {
		return nil, err
	}

	switch len(hpList) {
	case 0:
		return nil, nil
	case 1:
		httpProxy, err := contour.HTTPProxyFromUnstructured(hpList[0])
		if err != nil {
			return nil, err
		}
		return httpProxy, nil
	default:
		//TODO: modify HTTPProxy instead of Delete
		for _, hp := range hpList[1:] {
			log.Info("deleting HTTPProxy")
			err := s.DynamicClient.Resource(contour.HTTPProxyGvr()).Namespace(ch.Namespace).Delete(ctx, hp.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return nil, err
			}
		}
		return nil, fmt.Errorf("multiple HTTPProxies found")
	}
}

func (s *Solver) createHTTPProxy(ctx context.Context, ch *cmacme.Challenge, svcName string) (*contour.HTTPProxy, error) {
	expectedSpec := createHTTPProxySpec(ch, svcName)

	hp := contour.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "cm-acme-http-solver-",
			Namespace:       ch.Namespace,
			Annotations:     map[string]string{"kubernetes.io/ingress.class": "contour-public"},
			Labels:          podLabels(ch),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ch, challengeGvk)},
		},
		Spec: *expectedSpec,
	}

	unstr, err := hp.ToUnstructured()
	if err != nil {
		return nil, err
	}

	val, err := s.DynamicClient.Resource(contour.HTTPProxyGvr()).Namespace(ch.Namespace).Create(ctx, unstr, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	httpProxy, err := contour.HTTPProxyFromUnstructured(val)
	if err != nil {
		return nil, err
	}
	return httpProxy, nil
}

func (s *Solver) checkAndUpdateHTTPProxy(ctx context.Context, ch *cmacme.Challenge, svcName string, httpproxy *contour.HTTPProxy) (*contour.HTTPProxy, error) {
	log := logf.FromContext(ctx, "checkAndUpdateHTTPProxy")

	expectedSpec := createHTTPProxySpec(ch, svcName)
	spec := &httpproxy.Spec
	if reflect.DeepEqual(spec, expectedSpec) {
		return httpproxy, nil
	}

	log.Info("updating HTTPProxy")

	httpproxy.Spec = *expectedSpec
	unstr, err := httpproxy.ToUnstructured()
	if err != nil {
		return nil, err
	}
	val, err := s.DynamicClient.Resource(contour.HTTPProxyGvr()).Namespace(ch.Namespace).Update(ctx, unstr, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	httpProxy, err := contour.HTTPProxyFromUnstructured(val)
	if err != nil {
		return nil, err
	}
	return httpProxy, nil
}

func (s *Solver) cleanupHTTPProxy(_ context.Context, _ *cmacme.Challenge) error {
	// Nothing to do, GC will take care of deleting the HTTPProxy when the Challenge is deleted
	return nil
}

func createHTTPProxySpec(ch *cmacme.Challenge, svcName string) *contourv1.HTTPProxySpec {

	return &contourv1.HTTPProxySpec{
		VirtualHost: &contourv1.VirtualHost{Fqdn: ch.Spec.DNSName},
		Routes: []contourv1.Route{
			{
				Conditions: []contourv1.MatchCondition{
					{
						Prefix: fmt.Sprintf("/.well-known/acme-challenge/%s", ch.Spec.Token),
					},
				},
				Services: []contourv1.Service{
					{
						Name: svcName,
						Port: acmeSolverListenPort,
					},
				},
				PermitInsecure: true,
			},
		},
	}
}
