package worker

import (
	"context"
	"crypto/tls"
	"github.com/dgrr/http2"
	"github.com/valyala/fasthttp"
	"net/url"
	"sync"
	"time"
)

var (
	requestPool  *sync.Pool
	responsePool *sync.Pool
)

const (
	ReqBegin = 0
	ReqEnd   = 1
)

type TotalRequestsComplete int64

type Config struct {
	ReqURI           string
	DisableKeepAlive bool
	SkipVerify       bool
	MTLSKey          string
	MTLSCert         string
	Reqs             int64
	Ctx              context.Context
	StartTrigger     *sync.WaitGroup
	Until            time.Duration
	ReqEvery         time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	Method           string
	Verbose          bool
	HTTPV2           bool
}

type ResponseCode int

type ReqLatency [2]int64

type Stats struct {
	CompletedReqs int64
	FailedReqs    int64
	Reqs          []ReqLatency
	Responses     map[ResponseCode]int64
	Errors        map[string]uint
}

func (c *Config) TimeLimited() bool {
	return c.Until != 0
}

func (c *Config) UnlimitedReqs() bool {
	return c.Until != 0 && c.Reqs == 0
}

func NewWorker(config *Config) (Worker, error) {
	client, err := getClient(config)
	if err != nil {
		return nil, err
	}

	if responsePool == nil {
		responsePool = &sync.Pool{New: func() any {
			return &fasthttp.Response{}
		}}
	}

	if requestPool == nil {
		requestPool = &sync.Pool{New: func() any {
			req := &fasthttp.Request{}
			if config.DisableKeepAlive {
				req.Header.Add(fasthttp.HeaderConnection, "close")
			}
			if config.Method != "GET" {
				req.Header.SetMethodBytes([]byte(config.Method))
			}
			req.SetRequestURI(config.ReqURI)
			return req
		}}
	}

	if !config.TimeLimited() {
		return &WorkerFixedReqs{baseConfig(config, client)}, nil
	}

	if config.UnlimitedReqs() {
		return &WorkerFixedTime{baseConfig(config, client)}, nil
	}
	return &WorkerFixedTimeRequests{baseConfig(config, client)}, nil
}

func baseConfig(config *Config, client *fasthttp.HostClient) *WorkerBase {
	return &WorkerBase{
		config: config,
		client: client,
		stats: Stats{
			Responses: make(map[ResponseCode]int64),
			Errors:    make(map[string]uint),
		},
	}
}

func getClient(config *Config) (*fasthttp.HostClient, error) {
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

	u, err := url.ParseRequestURI(config.ReqURI)
	if err != nil {
		return nil, err
	}

	client := &fasthttp.HostClient{
		Addr:                          u.Host,
		IsTLS:                         u.Scheme == "https",
		MaxConns:                      1,
		ReadTimeout:                   config.ReadTimeout,
		WriteTimeout:                  config.WriteTimeout,
		DisableHeaderNamesNormalizing: true,
		TLSConfig:                     tlsConfig,
	}

	// TODO implement HTTPv3??? from github.com/quic-go/quic-go
	//if config.HTTPV3 {
	//	return &http.Client{
	//		Transport: &http3.RoundTripper{},
	//	}, nil
	//}

	if !config.HTTPV2 {
		return client, nil
	}

	if err := http2.ConfigureClient(client, http2.ClientOpts{
		MaxResponseTime: config.ReadTimeout,
	}); err != nil {
		return nil, err
	}

	return client, nil
}
