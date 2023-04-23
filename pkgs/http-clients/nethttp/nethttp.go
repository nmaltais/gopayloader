package nethttp

import (
	"bytes"
	"crypto/tls"
	"github.com/domsolutions/gopayloader/pkgs/http-clients"
	"github.com/quic-go/quic-go/http3"
	"io"
	"net/http"
)

type Client struct {
	client *http.Client
}

type Req struct {
	req *http.Request
}

type Resp struct {
	resp *http.Response
}

func (r *Resp) StatusCode() int {
	return r.resp.StatusCode
}

func (r *Req) SetHeader(key, val string) {
	r.req.Header.Set(key, val)
}

func (r *Req) SetMethod(method string) {
	r.req.Method = method
}

func (r *Req) SetBody(body []byte) {
	r.req.GetBody = func() (io.ReadCloser, error) {
		r := bytes.NewReader(body)
		return io.NopCloser(r), nil
	}
}

func (fh *Client) Do(req http_clients.Request, resp http_clients.Response) error {
	resptemp, err := fh.client.Do(req.(*Req).req)
	resp.(*Resp).resp = resptemp
	return err
}

func (fh *Client) NewResponse() http_clients.Response {
	return &Resp{}
}

func (fh *Client) NewReq(method, url string) (http_clients.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	return &Req{
		req: req,
	}, nil
}

func GetNetHTTPClient(config *http_clients.Config) (http_clients.GoPayLoaderClient, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.SkipVerify,
	}

	if config.MTLSCert != "" && config.MTLSKey != "" {
		cert, err := tls.LoadX509KeyPair(config.MTLSCert, config.MTLSKey)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return &Client{client: &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: config.ReadTimeout + config.WriteTimeout,
	}}, nil

}

func GetNetHTTP3Client(config *http_clients.Config) (http_clients.GoPayLoaderClient, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.SkipVerify,
	}

	if config.MTLSCert != "" && config.MTLSKey != "" {
		cert, err := tls.LoadX509KeyPair(config.MTLSCert, config.MTLSKey)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	roundTripper := &http3.RoundTripper{
		TLSClientConfig: tlsConfig,
	}

	return &Client{
		client: &http.Client{
			Transport: roundTripper,
		},
	}, nil
}
