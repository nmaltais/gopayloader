package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"encoding/json"
)

type Config struct {
	Ctx                  context.Context
	ReqURI               string
	DisableKeepAlive     bool
	ReqTarget            int64
	Conns                uint
	Duration             time.Duration
	MTLSKey              string
	MTLSCert             string
	SkipVerify           bool
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	Method               string
	Verbose              bool
	VerboseTicker        time.Duration
	JwtKID               string
	JwtKey               string
	JwtSub               string
	JwtCustomClaimsJSON  string
	JwtIss               string
	JwtAud               string
	JwtHeader            string
	JwtsFilename         string
	SendJWT              bool
	Headers              []string
	Body                 string
	BodyFile             string
	Client               string
}

func NewConfig(ctx context.Context, reqURI, mTLScert, mTLSKey string, disableKeepAlive bool, reqs int64, conns uint, totalTime time.Duration, skipVerify bool, readTimeout, writeTimeout time.Duration, method string, verbose bool, ticker time.Duration, jwtKID, jwtKey, jwtSub, jwtCustomClaimsJSON, jwtIss, jwtAud, jwtHeader, jwtsFilename string, headers []string, body, bodyFile string, client string) *Config {
	return &Config{
		Ctx:                 ctx,
		ReqURI:              reqURI,
		MTLSKey:             mTLSKey,
		MTLSCert:            mTLScert,
		DisableKeepAlive:    disableKeepAlive,
		ReqTarget:           reqs,
		Conns:               conns,
		Duration:            totalTime,
		SkipVerify:          skipVerify,
		ReadTimeout:         readTimeout,
		WriteTimeout:        writeTimeout,
		Method:              method,
		Verbose:             verbose,
		VerboseTicker:       ticker,
		JwtKID:              jwtKID,
		JwtKey:              jwtKey,
		JwtSub:              jwtSub,
		JwtCustomClaimsJSON: jwtCustomClaimsJSON,
		JwtIss:              jwtIss,
		JwtAud:              jwtAud,
		JwtHeader:           jwtHeader,
		JwtsFilename:        jwtsFilename,
		Headers:             headers,
		Body:                body,
		BodyFile:            bodyFile,
		Client:              client,
	}
}

var (
	errConnLimit = errors.New("connections can't be more than requests")
)

const regEx = `https?:\/\/(.)*(?::\d+)`

var regExHostURI = regexp.MustCompile(regEx)

var allowedMethods = [4]string{
	"GET",
	"PUT",
	"POST",
	"DELETE",
}

// Converts jwtCustomClaimsJSON from string to map[string]interface{}
func JwtCustomClaimsJSONStringToMap(jwtCustomClaimsJSON string) (map[string]interface{}, error) {
	if jwtCustomClaimsJSON == "" {
		return nil, nil
	}

	jwtCustomClaimsMap := map[string]interface{}{}

	err := json.Unmarshal([]byte(jwtCustomClaimsJSON), &jwtCustomClaimsMap)
	if err != nil {
		return nil, err
	}

	return jwtCustomClaimsMap, nil
}

func (c *Config) Validate() error {
	if _, err := url.ParseRequestURI(c.ReqURI); err != nil {
		return fmt.Errorf("config: invalid request uri, got error %v", err)
	}
	if int64(c.Conns) > c.ReqTarget && c.Duration == 0 {
		return errConnLimit
	}
	if int64(c.Conns) > c.ReqTarget && c.ReqTarget != 0 && c.Duration != 0 {
		return errConnLimit
	}
	if c.Conns == 0 {
		return errors.New("0 connections not allowed")
	}

	if !regExHostURI.MatchString(c.ReqURI) {
		return fmt.Errorf("url not in correct format %s needs to be like protocol://host:port/path i.e. https://localhost:443/some-path", c.ReqURI)
	}

	if c.MTLSKey != "" {
		_, err := os.OpenFile(c.MTLSKey, os.O_RDONLY, os.ModePerm)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("config: mTLS private key does not exist")
			}
			return fmt.Errorf("config: mTLS private key error checking file exists; %v", err)
		}
	}
	if c.MTLSCert != "" {
		_, err := os.OpenFile(c.MTLSCert, os.O_RDONLY, os.ModePerm)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("config: mTLS cert does not exist")
			}
			return fmt.Errorf("config: mTLS cert error checking file exists; %v", err)
		}
	}

	// Require JwtHeader if JwtKey or JwtsFilename is present
  if (c.JwtsFilename != "" || c.JwtKey != "") && c.JwtHeader == "" {
		return errors.New("config: empty jwt header")
	}

	// Require JwtKey or JwtsFilename if JwtHeader is present
	if c.JwtHeader != "" && c.JwtsFilename == "" && c.JwtKey == "" {
		return errors.New("config: empty jwt filename and jwt key, one of those is needed to send requests with JWTs")
	}

	if c.JwtKey != "" {
		_, err := os.OpenFile(c.JwtKey, os.O_RDONLY, os.ModePerm)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("config: jwt key does not exist")
			}
			return fmt.Errorf("config: jwt key error checking file exists; %v", err)
		}
		if c.ReqTarget == 0 {
			return errors.New("can only send jwts when request number is specified")
		}
		c.SendJWT = true
	}

	if c.JwtsFilename != "" {
		_, err := os.OpenFile(c.JwtsFilename, os.O_RDONLY, os.ModePerm)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("config: jwt file does not exist: " + c.JwtsFilename)
			}
			return fmt.Errorf("config: jwt file error checking file exists; %v", err)
		}
		if c.ReqTarget == 0 {
			return errors.New("can only send jwts when request number is specified")
		}
		c.SendJWT = true
	}

	if len(c.Headers) > 0 {
		for _, h := range c.Headers {
			if !strings.Contains(h, ":") {
				return fmt.Errorf("header %s does not contain : ", h)
			}
		}
	}

	if len(c.BodyFile) > 0 {
		_, err := os.OpenFile(c.BodyFile, os.O_RDONLY, os.ModePerm)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("config: body file does not exist")
			}
			return fmt.Errorf("config: body file error checking file exists; %v", err)
		}
	}

	if c.VerboseTicker == 0 {
		return errors.New("ticker value can't be zero")
	}

	if !methodAllowed(c.Method) {
		return fmt.Errorf("method %s not allowed", c.Method)
	}

	if c.WriteTimeout == 0 {
		return errors.New("write timeout is zero")
	}
	if c.ReadTimeout == 0 {
		return errors.New("read timeout is zero")
	}

	if c.ReqTarget == 0 && c.Duration == 0 {
		return errors.New("config: ReqTarget 0 and Duration 0")
	}

	if c.JwtCustomClaimsJSON != "" {
		_, err := JwtCustomClaimsJSONStringToMap(c.JwtCustomClaimsJSON)
		if err != nil {
			return fmt.Errorf("config: failed to parse custom json in --jwt-claims, got error; %v", err)
		}
	}

	return nil
}

func methodAllowed(method string) bool {
	for _, m := range allowedMethods {
		if method == m {
			return true
		}
	}
	return false
}
