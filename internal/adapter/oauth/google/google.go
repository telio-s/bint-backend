package google

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/telio-s/bint-backend.git/internal/domain/apperror"
	"github.com/telio-s/bint-backend.git/internal/domain/model"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type OAuth struct {
	logger   *slog.Logger
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

type idTokenClaims struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Nonce         string `json:"nonce"`
}

func New(logger *slog.Logger, config Config) *OAuth {
	keySet := oidc.NewRemoteKeySet(context.Background(), googleJWKSURL)
	return &OAuth{
		logger: logger.With("component", "google_oauth"),
		config: oauth2.Config{
			ClientID: config.ClientID, ClientSecret: config.ClientSecret,
			RedirectURL: config.RedirectURL, Endpoint: google.Endpoint,
			Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: oidc.NewVerifier(
			"https://accounts.google.com",
			keySet,
			&oidc.Config{ClientID: config.ClientID},
		),
	}
}

func (o *OAuth) AuthorizationURL(state, nonce, codeChallenge string) string {
	return o.config.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (o *OAuth) Exchange(
	ctx context.Context,
	code, codeVerifier, expectedNonce string,
) (model.GoogleIdentity, error) {
	token, err := o.config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		o.logger.WarnContext(ctx, "authorization code exchange failed", "error", err)
		return model.GoogleIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return model.GoogleIdentity{}, errors.New(
			"Google token response did not contain an ID token",
		)
	}
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		o.logger.WarnContext(ctx, "Google ID token verification failed", "error", err)
		return model.GoogleIdentity{}, fmt.Errorf("verify ID token: %w", err)
	}

	var claims idTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return model.GoogleIdentity{}, fmt.Errorf("decode ID token claims: %w", err)
	}
	if expectedNonce == "" ||
		subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		return model.GoogleIdentity{}, fmt.Errorf(
			"ID token nonce mismatch: %w",
			apperror.ErrInvalidToken,
		)
	}
	if claims.Subject == "" || claims.Email == "" {
		return model.GoogleIdentity{}, fmt.Errorf(
			"ID token is missing identity claims: %w",
			apperror.ErrInvalidToken,
		)
	}

	return model.GoogleIdentity{
		Subject: claims.Subject, Email: claims.Email, DisplayName: claims.Name,
		AvatarURL: claims.Picture, EmailVerified: claims.EmailVerified,
	}, nil
}
