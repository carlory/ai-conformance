package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type PrometheusQueryParams struct {
	RestClient            rest.Interface
	Config                *envconf.Config
	PrometheusURL         string
	PrometheusmNamespace  string
	PrometheusServiceName string
	Query                 string
}

func QueryPrometheus(params PrometheusQueryParams) (string, error) {
	if params.PrometheusmNamespace != "" && params.PrometheusServiceName != "" {
		req := params.RestClient.
			Get().
			RequestURI(fmt.Sprintf("/api/v1/namespaces/%s/services/%s:9090/proxy/api/v1/query", params.PrometheusmNamespace, params.PrometheusServiceName)).
			Param("query", params.Query)
		resp := req.Do(context.TODO())
		raw, err := resp.Raw()
		if err != nil {
			return "", err
		}
		return string(raw), nil
	} else if params.PrometheusURL != "" {
		u, err := url.Parse(params.PrometheusURL)
		if err != nil {
			return "", err
		}
		q := u.Query()
		q.Set("query", params.Query)
		u.RawQuery = q.Encode()
		resp, err := http.Get(u.String())
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}
	return "", errors.New("must specify either RestClient+PrometheusmNamespace+PrometheusServiceName or PrometheusURL")
}
