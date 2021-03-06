package gcp

import (
	"crypto/x509"
	"net/http"
	"os"
	"strings"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/access_token/file"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/common"
	authnConfig "github.com/cyberark/conjur-authn-k8s-client/pkg/authenticator/gcp/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/config"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/conjur-authn-k8s-client/pkg/utils"
)

const (
	// AuthnType is gcp for google cloud authentication
	AuthnType = "gcp"
)

// Authenticator contains the configuration and client
// for the authentication connection to Conjur
type Authenticator struct {
	client      *http.Client
	AccessToken access_token.AccessToken
	Config      authnConfig.Config
	PublicCert  *x509.Certificate
}

// GlobalConfig returns config used in the cmd package
func (auth *Authenticator) GlobalConfig() config.Config {
	return config.Config{
		TokenRefreshTimeout: auth.Config.TokenRefreshTimeout,
		ContainerMode:       auth.Config.ContainerMode,
	}
}

// Init returns config used in the cmd package
func (auth *Authenticator) Init() (common.Authenticator, string) {
	log.Debug(log.CAKC059)
	config, err := authnConfig.NewFromEnv()
	if err != nil {
		return nil, log.CAKC018
	}

	authn, err := New(*config)
	if err != nil {
		return nil, log.CAKC019
	}

	return authn, ""
}

// CanHandle returns true if provided string is 'gcp'
func (auth *Authenticator) CanHandle(authnType string) bool {
	return strings.ToLower(authnType) == AuthnType
}

// New creates a new authenticator instance from a token file
func New(config authnConfig.Config) (*Authenticator, error) {
	accessToken, err := file.NewAccessToken(config.TokenFilePath)
	if err != nil {
		return nil, log.RecordedError(log.CAKC001)
	}

	return NewWithAccessToken(config, accessToken)
}

// NewWithAccessToken creates a new authenticator instance from a given access token
func NewWithAccessToken(config authnConfig.Config, accessToken access_token.AccessToken) (*Authenticator, error) {
	client, err := newHTTPSClient(config.SSLCertificate)
	if err != nil {
		return nil, err
	}

	return &Authenticator{
		client:      client,
		AccessToken: accessToken,
		Config:      config,
	}, nil
}

// Authenticate sends Conjur an authenticate request and writes the response
// to the token file
func (auth *Authenticator) Authenticate() error {
	log.Info(log.CAKC040, auth.Config.Username)

	sessionToken, err := auth.sendMetadataRequest()
	if err != nil {
		return err
	}

	authenticationResponse, err := auth.sendAuthenticationRequest(sessionToken)
	if err != nil {
		return err
	}

	err = auth.AccessToken.Write(authenticationResponse)
	if err != nil {
		return err
	}

	log.Info(log.CAKC035)
	return nil
}

// sendAuthenticationRequest sends the google service account session token
// to the conjur authn url
func (auth *Authenticator) sendAuthenticationRequest(sessionToken []byte) ([]byte, error) {
	client, err := newHTTPSClient(auth.Config.SSLCertificate)
	if err != nil {
		return nil, err
	}

	base64Token := strings.ToLower(os.Getenv("CONJUR_BASE64_TOKEN")) == "true"

	req, err := AuthenticateRequest(
		auth.Config.URL,
		auth.Config.Account,
		sessionToken,
		base64Token,
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, log.RecordedError(log.CAKC027, err)
	}

	err = utils.ValidateResponse(resp)
	if err != nil {
		return nil, err
	}

	return utils.ReadResponseBody(resp)
}

// sendMetadataRequest sends the get google service account to the
// google metadata url and returns the service account session token
func (auth *Authenticator) sendMetadataRequest() ([]byte, error) {
	client, err := newHTTPSClient(auth.Config.SSLCertificate)
	if err != nil {
		return nil, err
	}

	req, err := MetadataRequest(
		auth.Config.Account,
		auth.Config.Username,
	)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, log.RecordedError(log.CAKC027, err)
	}

	err = utils.ValidateResponse(resp)
	if err != nil {
		return nil, err
	}

	return utils.ReadResponseBody(resp)
}
