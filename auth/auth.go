package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/metal-stack/metal-lib/zapup"

	"github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
)

// Config for parametrization
type Config struct {
	// url of the oidc endpoint
	IssuerURL     string `required:"true"`
	SkipTLSVerify bool
	IssuerRootCA  string

	// client identification
	ClientID     string `required:"true"`
	ClientSecret string `required:"true"`

	TLSCert string
	TLSKey  string

	// should a refresh token be requested if the server supports it?
	RequestRefreshToken bool

	TokenHandler TokenHandlerFunc `required:"true"`

	// Message shown on the success page after login flow
	SuccessMessage string

	Log *zap.Logger

	// Console if you want the library to write messages, may be nil
	Console io.Writer

	Debug bool
}

// TokenHandlerFunc function to handle the received token, e.g. write to file
type TokenHandlerFunc func(tokenInfo TokenInfo) error

type TokenInfo struct {
	IDToken      string
	RefreshToken string
	TokenClaims  claim

	IssuerConfig
}

// internal model
type app struct {
	config Config

	verifier *oidc.IDTokenVerifier
	provider *oidc.Provider

	state string

	// Does the provider use "offline_access" scope to request a refresh token
	// or does it use "access_type=offline" (e.g. Google)?
	offlineAsScope bool

	// local uri where the http-server should listen, e.g. http://localhost:5566
	Listen string

	// uri for redirects from oidc-endpoint after completion of oidc-flow, e.g. http://localhost:5566/callback
	RedirectURI string

	client       *http.Client
	completeChan chan bool
}

// logs to console if it is configured
func (a *app) Consolef(format string, args ...interface{}) {
	if a.config.Console != nil {
		fmt.Fprintf(a.config.Console, format, args...)
	}
}

type claim struct {
	Iss           string      `json:"iss"`
	Sub           string      `json:"sub"`
	Aud           interface{} `json:"aud"` // since it depends on the scopes if aud is a string or []string
	Exp           int         `json:"exp"`
	Iat           int         `json:"iat"`
	AtHash        string      `json:"at_hash"`
	Email         string      `json:"email"`
	EmailVerified bool        `json:"email_verified"`
	Name          string      `json:"name"`
}

// OIDCFlow validates the given config and starts the OIDC-Flow "response_type=code"
// (see https://medium.com/@darutk/diagrams-of-all-the-openid-connect-flows-6968e3990660
// or https://connect2id.com/learn/openid-connect).
//
// A local webserver is started to receive the callbacks from the oidc-endpoint.
//
// 1. OpenID Discovery --> gather info about OIDC Provider
// 2. open browser for login --> build url with scopes --> redirect to OIDC-Login-Flow (oidc-provider: auth with ldap, read groups, return signed jwt)
// 3. receive Callback, extract token and redirect to Success-Page
// 4. call TokenHandler
func OIDCFlow(config Config) error {

	if config.IssuerURL == "" {
		return errors.New("error validating config: IssuerURL is required")
	}

	if config.ClientID == "" {
		return errors.New("error validating config: ClientID is required")
	}

	if config.ClientSecret == "" {
		return errors.New("error validating config: ClientSecret is required")
	}

	if config.TokenHandler == nil {
		return errors.New("error validating config: TokenHandler is required")
	}

	if config.SkipTLSVerify && config.IssuerRootCA != "" {
		return errors.New("it makes no sense to use IssuerRootCA and SkipTLSVerify at the same time")
	}

	appModel := &app{
		config: config,
	}

	return oidcFlow(appModel)
}

func oidcFlow(appModel *app) error {

	if appModel.config.Log == nil {
		appModel.config.Log = zapup.MustRootLogger()
	}

	if appModel.config.SuccessMessage == "" {
		appModel.config.SuccessMessage = "Please close this page and return to your terminal."
	}

	if appModel.config.IssuerRootCA != "" {
		client, caerr := httpClientForRootCAs(appModel.config.IssuerRootCA)
		if caerr != nil {
			return caerr
		}
		appModel.client = client
	}

	if appModel.client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: appModel.config.SkipTLSVerify} // ignore expired SSL certificates

		appModel.client = &http.Client{
			Transport: transport,
		}
	}

	if appModel.config.Debug {
		appModel.client.Transport = debugTransport{roundTripper: appModel.client.Transport, log: appModel.config.Log}
	}

	// generate state
	appModel.state = uuid.New().String()

	clientCtx := oidc.ClientContext(context.Background(), appModel.client)

	provider, err := oidc.NewProvider(clientCtx, appModel.config.IssuerURL)
	if err != nil {
		return errors.Wrapf(err, "failed to query provider %q", appModel.config.IssuerURL)
	}

	var s struct {
		// What scopes does a provider support?
		//
		// See: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
		ScopesSupported []string `json:"scopes_supported"`
	}
	if err := provider.Claims(&s); err != nil {
		return errors.Wrapf(err, "failed to parse provider scopes_supported")
	}

	if len(s.ScopesSupported) == 0 {
		// scopes_supported is a "RECOMMENDED" discovery claim, not a required
		// one. If missing, assume that the provider follows the spec and has
		// an "offline_access" scope.
		appModel.offlineAsScope = true
	} else {
		// See if scopes_supported has the "offline_access" scope.
		appModel.offlineAsScope = func() bool {
			for _, scope := range s.ScopesSupported {
				if scope == oidc.ScopeOfflineAccess {
					return true
				}
			}
			return false
		}()
	}

	appModel.provider = provider
	appModel.verifier = provider.Verifier(&oidc.Config{ClientID: appModel.config.ClientID})
	appModel.completeChan = make(chan bool)

	// use next free port for callback
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	callbackPath := "/callback"

	appModel.config.Log.Debug("Listening", zap.String("hostname", "localhost"), zap.Int("port", port))

	srv := &http.Server{}
	http.HandleFunc("/", appModel.handleLogin)
	http.HandleFunc(callbackPath, appModel.handleCallback)

	appModel.Listen = fmt.Sprintf("http://localhost:%d", port)
	appModel.RedirectURI = fmt.Sprintf("%s%s", appModel.Listen, callbackPath)

	appModel.config.Log.Debug("Opening Browser for Authentication")

	appModel.Consolef("Opening Browser for Authentication. If this does not work, please point your browser to %s\n", appModel.Listen)

	go func() {
		err := openBrowser(appModel.Listen)
		if err != nil {
			appModel.config.Log.Error("openBrowser", zap.Error(err))
		}
	}()
	go func() {
		appModel.waitShutdown()
		err = srv.Shutdown(context.Background())
		if err != nil {
			appModel.config.Log.Error("Shutdown", zap.Error(err))
		}
	}()

	err = srv.Serve(listener)
	// after Shutdown ErrServerClosed is returned, this is expected and ok
	if err == http.ErrServerClosed {
		return nil
	}

	return err
}

// return an HTTP client which trusts the provided root CAs.
func httpClientForRootCAs(rootCAs string) (*http.Client, error) {
	tlsConfig := tls.Config{RootCAs: x509.NewCertPool()}
	rootCABytes, err := ioutil.ReadFile(rootCAs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read root-ca")
	}
	if !tlsConfig.RootCAs.AppendCertsFromPEM(rootCABytes) {
		return nil, errors.Errorf("no certs found in root CA file %q", rootCAs)
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}, nil
}

type debugTransport struct {
	roundTripper http.RoundTripper
	log          *zap.Logger
}

func (d debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}
	d.log.Debug("sending request", zap.ByteString("request", reqDump))

	resp, err := d.roundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	d.log.Debug("received response", zap.ByteString("response", respDump))
	return resp, nil
}

func (a *app) oauth2Config(scopes []string) *oauth2.Config {

	return &oauth2.Config{
		ClientID:     a.config.ClientID,
		ClientSecret: a.config.ClientSecret,
		Endpoint:     a.provider.Endpoint(),
		Scopes:       scopes,
		RedirectURL:  a.RedirectURI,
	}
}

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	var scopes []string

	var authCodeURL string

	scopes = append(scopes, "groups", "openid", "profile", "email", "federated:id")

	if a.config.RequestRefreshToken {
		if a.offlineAsScope {
			scopes = append(scopes, "offline_access")
			authCodeURL = a.oauth2Config(scopes).AuthCodeURL(a.state)
		} else {
			authCodeURL = a.oauth2Config(scopes).AuthCodeURL(a.state, oauth2.AccessTypeOffline)
		}
	} else {
		authCodeURL = a.oauth2Config(scopes).AuthCodeURL(a.state)
	}

	http.Redirect(w, r, authCodeURL, http.StatusSeeOther)
}

func (a *app) handleCallback(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		token *oauth2.Token
	)

	ctx := oidc.ClientContext(r.Context(), a.client)
	oauth2Config := a.oauth2Config(nil)
	switch r.Method {
	case "GET":
		// Authorization redirect callback from OAuth2 auth flow.
		if errMsg := r.FormValue("error"); errMsg != "" {
			http.Error(w, errMsg+": "+r.FormValue("error_description"), http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		if code == "" {
			http.Error(w, fmt.Sprintf("no code in request: %q", r.Form), http.StatusBadRequest)
			return
		}
		if state := r.FormValue("state"); state != a.state {
			http.Error(w, fmt.Sprintf("expected state %q got %q", a.state, state), http.StatusBadRequest)
			return
		}
		token, err = oauth2Config.Exchange(ctx, code)
	case "POST":
		// Form request from frontend to refresh a token.
		refresh := r.FormValue("refresh_token")
		if refresh == "" {
			http.Error(w, fmt.Sprintf("no refresh_token in request: %q", r.Form), http.StatusBadRequest)
			return
		}
		t := &oauth2.Token{
			RefreshToken: refresh,
			Expiry:       time.Now().Add(-time.Hour),
		}
		token, err = oauth2Config.TokenSource(ctx, t).Token()
	default:
		http.Error(w, fmt.Sprintf("method not implemented: %s", r.Method), http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in token response", http.StatusInternalServerError)
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to verify ID token: %v", err), http.StatusInternalServerError)
		return
	}
	var rawClaims json.RawMessage
	err = idToken.Claims(&rawClaims)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to parse claims: %v", err), http.StatusInternalServerError)
		return
	}

	buff := new(bytes.Buffer)
	err = json.Indent(buff, []byte(rawClaims), "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to indent json: %v", err), http.StatusInternalServerError)
		return
	}
	var claims claim
	err = json.Unmarshal(rawClaims, &claims)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read claims: %v", err), http.StatusInternalServerError)
		go func() {
			a.completeChan <- true
		}()
		return
	}

	if a.config.TokenHandler != nil {
		err = a.config.TokenHandler(TokenInfo{
			IDToken:      rawIDToken,
			RefreshToken: token.RefreshToken,
			TokenClaims:  claims,
			IssuerConfig: IssuerConfig{
				ClientID:     a.config.ClientID,
				ClientSecret: a.config.ClientSecret,
				IssuerURL:    a.config.IssuerURL,
				IssuerCA:     a.config.IssuerRootCA,
			},
		})

		if err != nil {
			a.config.Log.Error("error handling token", zap.Error(err))
		}
	}

	renderToken(w, rawIDToken, token.RefreshToken, buff.Bytes(), a.config.SuccessMessage, a.config.Debug)

	a.config.Log.Debug("Login Succeeded", zap.String("username", claims.Name))
	a.config.Log.Debug("Login-Data", zap.String("token", rawIDToken), zap.String("Refresh Token", token.RefreshToken), zap.String("Claims", string(rawClaims)))

	go func() {
		a.completeChan <- true
	}()
}

// waits for the token to be generated
func (a *app) waitShutdown() {
	<-a.completeChan
}

// Opens the given url in the browser (OS-dependent).
func openBrowser(url string) error {

	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	err := exec.Command(cmd, args...).Start()
	if err != nil {
		return fmt.Errorf("error opening browser cmd:%s args:%s error:%v", cmd, args, err)
	}
	return nil
}

// NewUpdateKubeConfigHandler writes the TokenInfo to file and prints a message to the given writer, may be nil
func NewUpdateKubeConfigHandler(kubeConfig string, writer io.Writer) TokenHandlerFunc {
	u := updateKubeConfig{
		kubeConfig:      kubeConfig,
		userIDExtractor: ExtractName,
		writer:          writer,
	}
	return u.updateKubeConfigFunc
}

type updateKubeConfig struct {
	// path to kubeconfig where the credentials should be written
	kubeConfig      string
	userIDExtractor UserIDExtractor
	//optional writer to print out messages
	writer io.Writer
}

func (u *updateKubeConfig) updateKubeConfigFunc(tokenInfo TokenInfo) error {

	filename, err := UpdateKubeConfig(u.kubeConfig, tokenInfo, u.userIDExtractor)
	if err != nil {
		return err
	}

	if u.writer != nil {
		fmt.Fprintf(u.writer, "Successfully written token to %s\n", filename)
	}

	return nil
}
