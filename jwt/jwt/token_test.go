package jwt

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
	"testing"
	"time"
)

func TestGenerateSimpleToken(t *testing.T) {

	alg := jose.RS256

	publicKey, privateKey, err := CreateWebkeyPair(alg, "sig")
	assert.NoError(t, err, "error creating keypair")

	cl := jwt.Claims{
		Subject:   "subject",
		Issuer:    "issuer",
		Expiry:    jwt.NewNumericDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		NotBefore: jwt.NewNumericDate(time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)),
		Audience:  jwt.Audience{"leela", "fry"},
	}

	signer := MustMakeSigner(alg, privateKey)

	token, err := CreateToken(signer, cl)
	assert.NoError(t, err, "error creating token")
	assert.NotEmpty(t, token)

	parsedClaims := &jwt.Claims{}
	webToken, err := jwt.ParseSigned(token)
	assert.NoError(t, err)
	err = webToken.Claims(publicKey, parsedClaims)
	assert.NoError(t, err, "error parsing claims")
	require.Equal(t, "subject", parsedClaims.Subject)
	require.Equal(t, "issuer", parsedClaims.Issuer)
}

func TestGenerateFullToken(t *testing.T) {

	alg := jose.RS256

	publicKey, privateKey, err := CreateWebkeyPair(alg, "sig")
	assert.NoError(t, err, "error creating keypair")

	cl := jwt.Claims{
		Issuer:   "https://dex.test.fi-ts.io/dex",
		Subject:  "achim",
		Audience: jwt.Audience{"token-forge", "auth-go-cli"},
		Expiry:   jwt.NewNumericDate(time.Unix(1557410799, 0)),
		IssuedAt: jwt.NewNumericDate(time.Unix(1557381999, 0)),
	}

	fed := map[string]string{
		"connector_id": "tenant_ldap_openldap",
		"user_id":      "cn=achim.admin,ou=People,dc=tenant,dc=de",
	}

	privateClaims := ExtendedClaims{
		Groups: []string{
			"k8s_kaas-admin",
			"k8s_kaas-edit",
			"k8s_kaas-view",
			"k8s_development__cluster-admin",
			"k8s_production__cluster-admin",
			"k8s_staging__cluster-admin",
		},
		EMail:           "achim.admin@tenant.de",
		Name:            "achim",
		FederatedClaims: fed,
	}

	signer := MustMakeSigner(alg, privateKey)

	token, err := CreateToken(signer, cl, privateClaims)
	assert.NoError(t, err, "error creating token")
	assert.NotEmpty(t, token)

	fmt.Println(token)
	bytes, err := publicKey.MarshalJSON()
	assert.NoError(t, err)
	fmt.Println(string(bytes))

	parsedClaims := &jwt.Claims{}
	webToken, err := jwt.ParseSigned(token)
	assert.NoError(t, err)
	err = webToken.Claims(publicKey, parsedClaims)
	assert.NoError(t, err, "error parsing claims")
}
