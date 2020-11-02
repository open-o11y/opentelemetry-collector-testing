package cortexexporter

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

// SigningRoundTripper is a Custom RoundTripper that performs AWS Sig V4
type SigningRoundTripper struct {
	transport http.RoundTripper
	signer    *v4.Signer
	cfg       *aws.Config
	params    AuthSettings
}

// RoundTrip signs each outgoing request
func (si *SigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBody, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	// Get the body
	content, err := ioutil.ReadAll(reqBody)
	if err != nil {
		return nil, err
	}

	body := bytes.NewReader(content)

	// Sign the request
	_, err = si.signer.Sign(req, body, si.params.Service, *si.cfg.Region, time.Now())
	if err != nil {
		return nil, err
	}
	if si.params.Debug {
		requestDump, err := httputil.DumpRequest(req, false)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(requestDump))
	}
	// Send the request to Cortex
	resp, err := si.transport.RoundTrip(req)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	if si.params.Debug {
		responseDump, err := httputil.DumpResponse(resp, false)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(responseDump))
	}

	return resp, err
}

// NewAuth takes a map of strings as parameters and return a http.RoundTripper that perform Sig V4 signing on each
// request.
func NewAuth(params AuthSettings, origClient *http.Client) (http.RoundTripper, error) {
	// check if region and service name are present
	err := validateAuthSettings(params)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Initialize session with default credential chain
	// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(params.Region)},
		aws.NewConfig().WithLogLevel(aws.LogDebugWithSigning),
	)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if _, err = sess.Config.Credentials.Get(); err != nil {
		log.Println(err)
		return nil, err
	}

	// Get Credentials, either from ./aws or from environmental variables
	creds := sess.Config.Credentials
	signer := v4.NewSigner(creds)
	if params.Debug {
		signer.Debug = aws.LogDebugWithSigning
		signer.Logger = aws.NewDefaultLogger()
	}
	rtp := SigningRoundTripper{
		transport: origClient.Transport,
		signer:    signer,
		cfg:       sess.Config,
		params:    params,
	}
	// return a RoundTripper
	return &rtp, nil
}
func validateAuthSettings(params AuthSettings) error {
	if params.Enabled && params.Region == "" || params.Service == "" {
		return fmt.Errorf("invalid authentication configuration")
	}
	return nil
}
