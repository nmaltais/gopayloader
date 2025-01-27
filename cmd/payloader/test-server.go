package payloader

import (
	"bufio"
	"crypto/tls"
	"errors"
	"github.com/quic-go/quic-go"
	httpv3server "github.com/quic-go/quic-go/http3"
	"github.com/spf13/cobra"
	"github.com/valyala/fasthttp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	port         int
	responseSize int
	fasthttp1    bool
	nethttp2     bool
	httpv3       bool
	debug        bool
)

var (
	serverCert string
	privateKey string
)

func init() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	serverCert = filepath.Join(wd, "/cmd/payloader/cert/server.crt")
	privateKey = filepath.Join(wd, "/cmd/payloader/cert/server.key")
}

func tlsConfig() *tls.Config {
	crt, err := os.ReadFile(serverCert)
	if err != nil {
		log.Fatal(err)
	}

	key, err := os.ReadFile(privateKey)
	if err != nil {
		log.Fatal(err)
	}

	cert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		log.Fatal(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   "localhost",
	}
}

var runServerCmd = &cobra.Command{
	Use:   "http-server",
	Short: "Start a local HTTP server",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		response := strings.Repeat("a", responseSize)
		addr := "localhost:" + strconv.Itoa(port)
		log.Println("Starting HTTP server on:", addr)

		if fasthttp1 {
			var err error

			server := fasthttp.Server{
				Handler: func(c *fasthttp.RequestCtx) {
					_, err = c.WriteString(response)
					if err != nil {
						log.Println(err)
					}
					if debug {
						log.Printf("%s\n", c.Request.Header.String())
						log.Printf("%s\n", c.Request.Body())
					}
				},
			}

			errs := make(chan error)
			go func() {
				if err := server.ListenAndServe(addr); err != nil {
					log.Println(err)
					errs <- err
				}
			}()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)

			select {
			case <-c:
				log.Println("User cancelled, shutting down")
				server.Shutdown()
			case err := <-errs:
				log.Printf("Got error from server; %v \n", err)
			}

			return nil
		}

		if nethttp2 {
			server := &http.Server{
				Addr:         addr,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
				TLSConfig:    tlsConfig(),
			}
			var err error

			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				_, err = w.Write([]byte(response))
				if err != nil {
					log.Println(err)
				}
				if debug {
					log.Printf("%+v\n", r.Header.Get("Some-Jwt"))
				}
			})

			if err := server.ListenAndServeTLS("", ""); err != nil {
				log.Fatal(err)
			}
		}

		if httpv3 {
			var err error

			quicConf := &quic.Config{
				EnableDatagrams: true,
			}

			tlsConfigServer := tlsConfig()

			server := httpv3server.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err = w.Write([]byte(response))
					if err != nil {
						log.Println(err)
					}
					if debug {
						log.Printf("%+v\n", r.Header)
					}
				}),
				Addr:       addr,
				QuicConfig: quicConf,
				TLSConfig:  tlsConfigServer,
			}

			if err := server.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}

		return errors.New("http option not recognised")
	},
}

func init() {
	runServerCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port")
	runServerCmd.Flags().IntVarP(&responseSize, "response-size", "s", 10, "Response size")
	runServerCmd.Flags().BoolVar(&fasthttp1, "fasthttp-1", false, "Fasthttp HTTP/1.1 server")
	runServerCmd.Flags().BoolVar(&nethttp2, "netHTTP-2", false, "net/http HTTP/2 server")
	runServerCmd.Flags().BoolVar(&httpv3, "http-3", false, "HTTP/3 server")
	runServerCmd.Flags().BoolVarP(&debug, "verbose", "v", false, "print logs")
	rootCmd.AddCommand(runServerCmd)
}

type MyWriteCloser struct {
	*bufio.Writer
}

func (mwc *MyWriteCloser) Close() error {
	// Noop
	return nil
}
