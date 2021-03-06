package auth

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func Test_GetCurrentUser(t *testing.T) {

	tests := []test{
		{
			filename: "./testdata/config",
			validate: expectSuccess(
				TestAuthContext{
					User:             "IZT0322",
					Ctx:              cloudContextName,
					AuthProviderName: "oidc",
					AuthProviderOidc: true,
					IDToken:          "eyJhbGciOiJSUzI1NiIsImtpZCI6IjFlNzNiYzJkM2IyN2FlODdiNDI4OWYzODk4ZjE3YmI4YmZlOGQ4N2IifQ.eyJpc3MiOiJodHRwczovL2RleC50ZXN0LmZpLXRzLmlvL2RleCIsInN1YiI6IkNrdERUajFKV2xRd016SXlMRTlWUFZWemNsTjJZeXhQVlQxVmMzSkJiR3dzVDFVOVNVUk5MRTlWUFVObGJuUnlZV3dzUkVNOWRHVnpkQzFqZFhOMGIyMWxjaXhFUXoxa2IyMXBiblFTREdGa2RHVnpkREZmWm1sMGN3IiwiYXVkIjpbInRva2VuLWZvcmdlIiwiYXV0aC1nby1jbGkiXSwiZXhwIjoxNTU2NjUwMDAwLCJpYXQiOjE1NTY2MjEyMDAsImF6cCI6ImF1dGgtZ28tY2xpIiwiYXRfaGFzaCI6Ik05eWlRRTlnLVB4eHFhR0diUDl0SGciLCJlbWFpbCI6IklaVDAzMjJAdGVzdC1jdXN0b21lci5kb21pbnQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6IklaVDAzMjIiLCJmZWRlcmF0ZWRfY2xhaW1zIjp7ImNvbm5lY3Rvcl9pZCI6ImFkdGVzdDFfZml0cyIsInVzZXJfaWQiOiJDTj1JWlQwMzIyLE9VPVVzclN2YyxPVT1Vc3JBbGwsT1U9SURNLE9VPUNlbnRyYWwsREM9dGVzdC1jdXN0b21lcixEQz1kb21pbnQifX0.KDHf2PF21tBFiNTyaMTQzJrs7JDJt8v4P5t5YOz3jLS1V3G6EueVgSY-bpl1VN16AmWyZ14Xj6fG7GZCxQGVW1NwHDZAi6IaOJmSLcjukj-jzwK6SjuRd8TIwuB5PepqUHGwG9AU6HoDQ5cLLuCYzn-CRUt-HB0uu6QBeznnmRT4VevbxHubxQFdui-ElReq-9R3KzoE-j6EPIoA2WQzA-PFeOvgZCBtYRC2tmTibObUaS7F1cz0cH0PnrpqkJ1_Lg91amcv-bUXRF1yWthKFNIQ9N9L7JqcCUYYVS2V2GG3pTo7ljoPfSBDybXe00BQjAM-EbrDeaplKl8ypOIdZg",
				}),
		},
		{
			filename: "./testdata/config-bare",
			validate: expectSuccess(
				TestAuthContext{
					User:             "IZT0322",
					Ctx:              cloudContextName,
					AuthProviderName: "oidc",
					AuthProviderOidc: true,
					IDToken:          "eyJhbGciOiJSUzI1NiIsImtpZCI6IjFlNzNiYzJkM2IyN2FlODdiNDI4OWYzODk4ZjE3YmI4YmZlOGQ4N2IifQ.eyJpc3MiOiJodHRwczovL2RleC50ZXN0LmZpLXRzLmlvL2RleCIsInN1YiI6IkNrdERUajFKV2xRd016SXlMRTlWUFZWemNsTjJZeXhQVlQxVmMzSkJiR3dzVDFVOVNVUk5MRTlWUFVObGJuUnlZV3dzUkVNOWRHVnpkQzFqZFhOMGIyMWxjaXhFUXoxa2IyMXBiblFTREdGa2RHVnpkREZmWm1sMGN3IiwiYXVkIjpbInRva2VuLWZvcmdlIiwiYXV0aC1nby1jbGkiXSwiZXhwIjoxNTU2NjUwMDAwLCJpYXQiOjE1NTY2MjEyMDAsImF6cCI6ImF1dGgtZ28tY2xpIiwiYXRfaGFzaCI6Ik05eWlRRTlnLVB4eHFhR0diUDl0SGciLCJlbWFpbCI6IklaVDAzMjJAdGVzdC1jdXN0b21lci5kb21pbnQiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6IklaVDAzMjIiLCJmZWRlcmF0ZWRfY2xhaW1zIjp7ImNvbm5lY3Rvcl9pZCI6ImFkdGVzdDFfZml0cyIsInVzZXJfaWQiOiJDTj1JWlQwMzIyLE9VPVVzclN2YyxPVT1Vc3JBbGwsT1U9SURNLE9VPUNlbnRyYWwsREM9dGVzdC1jdXN0b21lcixEQz1kb21pbnQifX0.KDHf2PF21tBFiNTyaMTQzJrs7JDJt8v4P5t5YOz3jLS1V3G6EueVgSY-bpl1VN16AmWyZ14Xj6fG7GZCxQGVW1NwHDZAi6IaOJmSLcjukj-jzwK6SjuRd8TIwuB5PepqUHGwG9AU6HoDQ5cLLuCYzn-CRUt-HB0uu6QBeznnmRT4VevbxHubxQFdui-ElReq-9R3KzoE-j6EPIoA2WQzA-PFeOvgZCBtYRC2tmTibObUaS7F1cz0cH0PnrpqkJ1_Lg91amcv-bUXRF1yWthKFNIQ9N9L7JqcCUYYVS2V2GG3pTo7ljoPfSBDybXe00BQjAM-EbrDeaplKl8ypOIdZg",
				}),
		},
		{
			filename: "./testdata/config-no-oidc",
			validate: expectError("missing key: auth-provider (path element idx: 1)"),
		},
		{
			filename: "./testdata/config-notexists",
			validate: expectError("error loading kube-config: stat ./testdata/config-notexists: no such file or directory"),
		},
		{
			filename: "./testdata/config-empty",
			validate: expectError("error loading kube-config - config is empty"),
		},
	}

	for _, currentTest := range tests {
		t.Run(currentTest.filename, func(t *testing.T) {

			authCtx, err := CurrentAuthContext(currentTest.filename)
			validateErr := currentTest.validate(t, authCtx, err)
			if validateErr != nil {
				t.Errorf("test failed with unexpected error: %v", validateErr)
			}
		})
	}
}

type TestAuthContext AuthContext

func (tac *TestAuthContext) compare(t *testing.T, authCtx AuthContext) {

	if authCtx.User != tac.User {
		t.Errorf("expected user %s", tac.User)
	}
	if authCtx.Ctx != tac.Ctx {
		t.Errorf("expected ctx %s", tac.Ctx)
	}
	if authCtx.AuthProviderName != tac.AuthProviderName {
		t.Errorf("expected authProviderName %s", tac.AuthProviderName)
	}
	if authCtx.AuthProviderOidc != tac.AuthProviderOidc {
		t.Errorf("expected oidc %t", tac.AuthProviderOidc)
	}
	if authCtx.IDToken != tac.IDToken {
		t.Errorf("expected idtoken %s", tac.IDToken)
	}
}

type test struct {
	filename string
	validate validateFn
}

type validateFn func(t *testing.T, ctx AuthContext, err error) error

type successData struct {
	expected TestAuthContext
}

func expectSuccess(expected TestAuthContext) validateFn {
	s := successData{
		expected: expected,
	}

	return s.validateSuccess
}

func (s *successData) validateSuccess(t *testing.T, authCtx AuthContext, err error) error {

	if err != nil {
		return err
	}

	s.expected.compare(t, authCtx)

	return nil
}

func expectError(errorMsg string) validateFn {
	e := errorData{
		errorMessage: errorMsg,
	}

	return e.validateError
}

type errorData struct {
	errorMessage string
}

func (e *errorData) validateError(t *testing.T, ctx AuthContext, err error) error {

	if err == nil {
		return fmt.Errorf("expected error '%s', got none", e.errorMessage)
	}

	if err.Error() != e.errorMessage {
		return fmt.Errorf("expected error '%s', got '%s'", e.errorMessage, err.Error())
	}

	return nil
}

var demoToken = TokenInfo{
	IssuerConfig: IssuerConfig{
		ClientID:     "clientId_abcd",
		ClientSecret: "clientSecret_123123",
		IssuerURL:    "the_issuer",
		IssuerCA:     "/my/ca",
	},
	TokenClaims: claim{
		Iss:   "the_issuer",
		Email: "email@provider.de",
		Sub:   "the_sub",
		Name:  "iz00001",
	},
	IDToken:      "abcd4711",
	RefreshToken: "refresh234",
}

var demoToken2 = TokenInfo{
	IssuerConfig: IssuerConfig{
		ClientID:     "clientId_abcd",
		ClientSecret: "clientSecret_123123",
		IssuerURL:    "the_issuer",
		IssuerCA:     "/my/ca",
	},
	TokenClaims: claim{
		Iss:   "the_issuer",
		Email: "other-email@other-provider.de",
		Sub:   "the_sub",
		Name:  "iz00002",
	},
	IDToken:      "cdefg",
	RefreshToken: "refresh987",
}

func TestUpdateUserNewFile(t *testing.T) {

	asserter := require.New(t)

	tmpfileName := filepath.Join(os.TempDir(), fmt.Sprintf("this_file_must_not_exist_%d", rand.Int63()))

	// delete file, just to be sure
	_ = os.Remove(tmpfileName)

	// "Update" -> create new file
	ti := demoToken
	_, err := UpdateKubeConfig(tmpfileName, ti, ExtractEMail)
	if err != nil {
		t.Fatalf("error updating kube-config: %v", err)
	}

	defer os.Remove(tmpfileName)

	// check it is written
	asserter.FileExists(tmpfileName, "expected file to exist")

	// check contents
	diffFiles(t, "./testdata/createdDemoConfig", tmpfileName)

	authContext, err := CurrentAuthContext(tmpfileName)
	if err != nil {
		t.Fatalf("error reading back user: %v", err)
	}

	asserter.Equal(authContext.User, demoToken.TokenClaims.Email, "User")
	asserter.Equal(authContext.IDToken, demoToken.IDToken, "IDToken")
	asserter.Equal(authContext.AuthProviderName, "oidc", "AuthProvider")
	asserter.Equal(authContext.Ctx, cloudContextName, "Context")
	asserter.Equal(authContext.ClientID, demoToken.ClientID, "ClientID")
	asserter.Equal(authContext.ClientSecret, demoToken.ClientSecret, "ClientSecret")
	asserter.Equal(authContext.IssuerURL, demoToken.IssuerURL, "Issuer")
	asserter.Equal(authContext.IssuerCA, demoToken.IssuerCA, "IssuerCA")

}

func TestUpdateUserWithNameExtractorNewFile(t *testing.T) {

	asserter := require.New(t)

	tmpfileName := filepath.Join(os.TempDir(), fmt.Sprintf("this_file_must_not_exist_%d", rand.Int63()))

	// delete file, just to be sure
	_ = os.Remove(tmpfileName)

	// "Update" -> create new file
	ti := demoToken
	_, err := UpdateKubeConfig(tmpfileName, ti, ExtractName)
	if err != nil {
		t.Fatalf("error updating kube-config: %v", err)
	}

	defer os.Remove(tmpfileName)

	// check it is written
	asserter.FileExists(tmpfileName, "expected file to ")

	// check contents
	diffFiles(t, "./testdata/createdDemoConfigName", tmpfileName)

	authContext, err := CurrentAuthContext(tmpfileName)
	if err != nil {
		t.Fatalf("error reading back user: %v", err)
	}

	asserter.Equal(authContext.User, demoToken.TokenClaims.Name, "User")
	asserter.Equal(authContext.IDToken, demoToken.IDToken, "IDToken")
	asserter.Equal(authContext.ClientID, demoToken.ClientID, "ClientID")
	asserter.Equal(authContext.ClientSecret, demoToken.ClientSecret, "ClientSecret")
	asserter.Equal(authContext.IssuerURL, demoToken.IssuerURL, "Issuer")
	asserter.Equal(authContext.IssuerCA, demoToken.IssuerCA, "IssuerCA")
	asserter.Equal(authContext.AuthProviderName, "oidc", "AuthProvider")
	asserter.Equal(authContext.Ctx, cloudContextName, "Context")
}

func TestLoadExistingConfigWithOIDC(t *testing.T) {

	authContext, err := CurrentAuthContext("./testdata/UEMCgivenConfig")

	require.NoError(t, err)

	require.Equal(t, authContext.User, demoToken.TokenClaims.Email, "User")
	require.Equal(t, authContext.IDToken, demoToken.IDToken, "IDToken")
	require.Equal(t, authContext.ClientID, demoToken.ClientID, "ClientID")
	require.Equal(t, authContext.ClientSecret, demoToken.ClientSecret, "ClientSecret")
	require.Equal(t, authContext.IssuerURL, demoToken.IssuerURL, "Issuer")
	require.Equal(t, authContext.IssuerCA, demoToken.IssuerCA, "IssuerCA")
	require.Equal(t, authContext.AuthProviderName, "oidc", "AuthProvider")
	require.Equal(t, authContext.Ctx, cloudContextName, "Context")
}

func TestUpdateUserExistingConfig(t *testing.T) {

	tmpfile := writeTemplate(t, "./testdata/UEUgivenConfig")
	defer os.Remove(tmpfile.Name()) // clean up

	_, err := UpdateKubeConfig(tmpfile.Name(), demoToken, ExtractEMail)
	if err != nil {
		t.Fatalf("error updating config: %v", err)
	}

	diffFiles(t, "./testdata/UEUexpectedConfig", tmpfile.Name())
}

func TestUpdateExistingMetalctlConfig(t *testing.T) {

	tmpfile := writeTemplate(t, "./testdata/UEMCgivenConfig")
	defer os.Remove(tmpfile.Name()) // clean up

	_, err := UpdateKubeConfig(tmpfile.Name(), demoToken2, ExtractEMail)
	if err != nil {
		t.Fatalf("error updating config: %v", err)
	}

	diffFiles(t, "./testdata/UEMCexpectedConfig", tmpfile.Name())

	_, err = UpdateKubeConfig(tmpfile.Name(), demoToken2, ExtractEMail)
	if err != nil {
		t.Fatalf("error updating config: %v", err)
	}

	diffFiles(t, "./testdata/UEMCexpectedConfig", tmpfile.Name())
}

func TestManipulateEncodeKubeconfig(t *testing.T) {

	// load full kubeconfig
	cfg, _, _, err := LoadKubeConfig("./testdata/UEUgivenConfig")
	require.NoError(t, err)

	err = AddUser(cfg, AuthContext{
		Ctx:              "user",
		User:             "username",
		AuthProviderName: "authprovider",
		AuthProviderOidc: true,
		IDToken:          "1234",
		RefreshToken:     "5678",
		IssuerConfig: IssuerConfig{
			ClientID:     "clientdId123",
			ClientSecret: "clientSecret345",
			IssuerURL:    "https://issuer",
			IssuerCA:     "/ca.cert",
		},
	})
	require.NoError(t, err)

	clusters, err := GetClusterNames(cfg)
	require.NoError(t, err)
	require.Equal(t, 1, len(clusters))

	err = AddContext(cfg, "myContext", clusters[0], "username")
	require.NoError(t, err)
	SetCurrentContext(cfg, "myContext")

	// encode result
	buf, err := EncodeKubeconfig(cfg)
	require.NoError(t, err)

	want, err := ioutil.ReadFile("./testdata/UEUManipulatedExpectedConfig")
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(want, buf.Bytes()))
}

func TestReduceAndEncodeKubeconfig(t *testing.T) {

	// load full kubeconfig
	cfg, _, _, err := LoadKubeConfig("./testdata/UEMCgivenConfig")
	require.NoError(t, err)

	// create empty kubeconfig
	resultCfg := make(map[interface{}]interface{})
	err = CreateFromTemplate(&resultCfg)
	require.NoError(t, err)

	// copy over clusters only
	resultCfg["clusters"] = cfg["clusters"]

	// encode result
	buf, err := EncodeKubeconfig(resultCfg)
	require.NoError(t, err)

	want, err := ioutil.ReadFile("./testdata/UEUReducedExpectedConfig")
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(want, buf.Bytes()))
}

func TestKubeconfigFromEnv(t *testing.T) {

	tmpfile := writeTemplate(t, "./testdata/UEMCgivenConfig")
	defer os.Remove(tmpfile.Name()) // clean up

	os.Setenv(RecommendedConfigPathEnvVar, tmpfile.Name())
	defer os.Setenv(RecommendedConfigPathEnvVar, "")

	_, filename, isDefault, err := LoadKubeConfig("")
	require.Nil(t, err)
	require.Equal(t, tmpfile.Name(), filename)
	require.False(t, isDefault)
}

func TestAuthContextFromEnv(t *testing.T) {

	tmpfile := writeTemplate(t, "./testdata/UEMCgivenConfig")
	defer os.Remove(tmpfile.Name()) // clean up

	os.Setenv(RecommendedConfigPathEnvVar, tmpfile.Name())
	defer os.Setenv(RecommendedConfigPathEnvVar, "")

	authCtx, err := CurrentAuthContext("")
	require.Nil(t, err)
	require.Equal(t, cloudContextName, authCtx.Ctx)
	require.Equal(t, "email@provider.de", authCtx.User)
}

func TestKubeconfigDefault(t *testing.T) {

	// TODO we can't control the default location without mocking the fileaccess
	// it would be good to test the "path will be created if default location does not exist" feature
	_, _, isDefault, _ := LoadKubeConfig("")
	require.True(t, isDefault)
}

func TestKubeconfigFromEnvDoesNotExist(t *testing.T) {

	os.Setenv(RecommendedConfigPathEnvVar, "/tmp/path/to/kubeconfig")
	defer os.Setenv(RecommendedConfigPathEnvVar, "")

	authCtx, filename, isDefault, err := LoadKubeConfig("")
	require.Nil(t, err)
	require.Equal(t, "/tmp/path/to/kubeconfig", filename)
	require.NotNil(t, authCtx)
	require.False(t, isDefault)
}

func TestAuthContextFromEnvDoesNotExist(t *testing.T) {

	tmpfile := writeTemplate(t, "./testdata/UEMCgivenConfig")
	defer os.Remove(tmpfile.Name()) // clean up

	os.Setenv(RecommendedConfigPathEnvVar, tmpfile.Name())
	defer os.Setenv(RecommendedConfigPathEnvVar, "")

	_, err := CurrentAuthContext("")
	require.Nil(t, err)
}

func TestKubeconfigFromEnvMultiplePaths(t *testing.T) {

	os.Setenv(RecommendedConfigPathEnvVar, "/tmp/path/to/kubeconfig:/another/path")
	defer os.Setenv(RecommendedConfigPathEnvVar, "")

	_, filename, isDefault, err := LoadKubeConfig("")
	require.EqualError(t, err, "there are multiple files in env KUBECONFIG, don't know which one to update - please use cmdline-option")
	require.Equal(t, "", filename)
	require.False(t, isDefault)
}

func writeTemplate(t *testing.T, templateName string) (f *os.File) {
	tmpfile, err := ioutil.TempFile("", "test-template")
	if err != nil {
		t.Fatalf("error creating empty template: %v", err)
	}

	var template []byte
	template, err = ioutil.ReadFile(templateName)
	if err != nil {
		t.Fatalf("error reading template: %v", err)
	}

	if _, err := tmpfile.Write(template); err != nil {
		t.Fatalf("error writing template: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("error closing template: %v", err)
	}

	return tmpfile
}

// diff given file contents and report diff-errors as t.Error
func diffFiles(t *testing.T, expectedFileName string, gotFileName string) {

	var err error

	var gotBytes []byte
	gotBytes, err = ioutil.ReadFile(gotFileName)
	if err != nil {
		t.Fatalf("error reading created file: %v", err)
	}

	var expectedBytes []byte
	expectedBytes, err = ioutil.ReadFile(expectedFileName)
	if err != nil {
		t.Fatalf("error reading expected data file: %v", err)
	}

	if diff := cmp.Diff(expectedBytes, gotBytes); diff != "" {
		t.Errorf("output differs (-want +got)\n%s", diff)
		t.Log(string(gotBytes))
	}
}
