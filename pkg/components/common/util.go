// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/readonlyfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	helmv1alpha1 "github.com/gardener/landscaper/apis/deployer/helm/v1alpha1"
	"github.com/gardener/landscaper/pkg/api"
	"github.com/gardener/landscaper/pkg/components/model/types"
	"github.com/gardener/landscaper/pkg/utils/blueprints"
)

var scheme = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9]*://.*$")

func NormalizeUrl(url string) string {
	result := strings.TrimSpace(url)
	result = strings.TrimSuffix(result, "/")
	if !scheme.MatchString(result) {
		return "https://" + result
	}
	return result
}

func GetAuthHeader(ctx context.Context, authData *helmv1alpha1.Auth, lsClient client.Client, namespace string) (string, error) {
	if len(authData.AuthHeader) > 0 && authData.SecretRef != nil {
		return "", fmt.Errorf("failed to get auth header: auth header and secret ref are both set")
	}

	if len(authData.AuthHeader) > 0 {
		return authData.AuthHeader, nil
	}

	if authData.SecretRef != nil {
		secretKey := client.ObjectKey{Name: authData.SecretRef.Name, Namespace: namespace}
		secret := &corev1.Secret{}
		if err := lsClient.Get(ctx, secretKey, secret); err != nil {
			return "", err
		}

		authHeaderKey := authData.SecretRef.Key
		if len(authData.SecretRef.Key) == 0 {
			authHeaderKey = types.AuthHeaderSecretDefaultKey
		}

		authHeader, ok := secret.Data[authHeaderKey]
		if !ok {
			return "", fmt.Errorf("failed to get auth header: key %s not found in secret", authHeaderKey)
		}

		return string(authHeader), nil
	}

	return "", fmt.Errorf("failed to get auth header: neither auth header nor secret ref is set")
}

// Copied from standard http module

// ParseBasicAuth parses an HTTP Basic Authentication string.
func ParseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !EqualFold(auth[:len(prefix)], prefix) {
		return "", "", false
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", "", false
	}
	cs := string(c)
	username, password, ok = strings.Cut(cs, ":")
	if !ok {
		return "", "", false
	}
	return username, password, true
}

// EqualFold is strings.EqualFold, ASCII only. It reports whether s and t
// are equal, ASCII-case-insensitively.
func EqualFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if lower(s[i]) != lower(t[i]) {
			return false
		}
	}
	return true
}

// lower returns the ASCII lowercase version of b.
func lower(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// BuildBlueprintFromPath creates a read-only blueprint from an extracted blueprint.
func BuildBlueprintFromPath(fs vfs.FileSystem, bpPath string) (*blueprints.Blueprint, error) {
	blueprintBytes, err := vfs.ReadFile(fs, filepath.Join(bpPath, lsv1alpha1.BlueprintFileName))
	if err != nil {
		return nil, fmt.Errorf("unable to read blueprint definition: %w", err)
	}
	blueprint := &lsv1alpha1.Blueprint{}
	if _, _, err := api.Decoder.Decode(blueprintBytes, nil, blueprint); err != nil {
		return nil, fmt.Errorf("unable to decode blueprint definition: %w", err)
	}
	bpFs, err := projectionfs.New(readonlyfs.New(fs), bpPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create blueprint filesystem: %w", err)
	}
	return blueprints.New(blueprint, readonlyfs.New(bpFs)), nil
}
