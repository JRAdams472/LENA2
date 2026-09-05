// Command testissuer is a standalone OIDC issuer for end-to-end tests.
// It serves a discovery document and JWKS for a randomly generated RSA key
// and mints signed ID tokens on demand. It is intended only for local e2e
// runs (docker-compose.e2e.yml); never deploy it.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("PORT", "8085")
	issuer := env("ISSUER_URL", "http://localhost:"+port)
	audience := env("TOKEN_AUDIENCE", "lena-e2e-client")

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generate rsa key: %v", err)
	}
	const keyID = "e2e-key"

	pub, err := jwk.Import(priv.Public())
	if err != nil {
		log.Fatalf("import public key: %v", err)
	}
	_ = pub.Set(jwk.KeyIDKey, keyID)
	_ = pub.Set(jwk.AlgorithmKey, jwa.RS256())
	_ = pub.Set(jwk.KeyUsageKey, "sig")
	jwks := jwk.NewSet()
	_ = jwks.AddKey(pub)

	signingKey, err := jwk.Import(priv)
	if err != nil {
		log.Fatalf("import private key: %v", err)
	}
	_ = signingKey.Set(jwk.KeyIDKey, keyID)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{
			"issuer":   issuer,
			"jwks_uri": issuer + "/jwks",
		})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, jwks)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// GET /token?sub=&email=&name=&aud= mints a signed ID token.
	mux.HandleFunc("GET /token", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sub := q.Get("sub")
		if sub == "" {
			sub = "e2e-subject"
		}
		aud := q.Get("aud")
		if aud == "" {
			aud = audience
		}
		now := time.Now()
		tok, err := jwt.NewBuilder().
			Issuer(issuer).
			Audience([]string{aud}).
			Subject(sub).
			IssuedAt(now).
			Expiration(now.Add(time.Hour)).
			Claim("email", q.Get("email")).
			Claim("name", q.Get("name")).
			Build()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), signingKey))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"id_token": string(signed)})
	})

	log.Printf("testissuer listening on :%s (issuer=%s, audience=%s)", port, issuer, audience)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
