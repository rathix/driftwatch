package fetcher

import (
	"context"
	"strings"
	"time"

	"github.com/kennyandries/driftwatch/pkg/types"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"golang.org/x/time/rate"
)

type ResourceMapper interface {
	ResourceFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error)
}

type Fetcher struct {
	Client  dynamic.Interface
	Mapper  ResourceMapper
	Limiter *rate.Limiter
	Timeout time.Duration
}

func NewFetcher(client dynamic.Interface, mapper ResourceMapper) *Fetcher {
	return &Fetcher{
		Client:  client,
		Mapper:  mapper,
		Limiter: rate.NewLimiter(10, 10),
		Timeout: 10 * time.Second,
	}
}

func (f *Fetcher) Get(ctx context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error) {
	if err := f.Limiter.Wait(ctx); err != nil {
		return nil, err
	}

	gvk, err := parseGVK(id.APIVersion, id.Kind)
	if err != nil {
		return nil, err
	}

	gvr, err := f.Mapper.ResourceFor(gvk)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, f.Timeout)
	defer cancel()

	obj, err := f.Client.Resource(gvr).Namespace(id.Namespace).Get(ctx, id.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return obj, nil
}

func IsNotFound(err error) bool {
	return k8serrors.IsNotFound(err)
}

func parseGVK(apiVersion, kind string) (schema.GroupVersionKind, error) {
	parts := strings.SplitN(apiVersion, "/", 2)
	var group, version string
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		group = ""
		version = parts[0]
	}
	return schema.GroupVersionKind{Group: group, Version: version, Kind: kind}, nil
}
