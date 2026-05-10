// Package imports ensures all required dependencies are included in go.mod
// This file will be removed once the dependencies are actually used in the codebase
package internal

import (
	// Web framework
	_ "github.com/gin-gonic/gin"

	// Kubernetes client
	_ "k8s.io/client-go/kubernetes"

	// Redis client
	_ "github.com/redis/go-redis/v9"

	// OAuth2
	_ "golang.org/x/oauth2"
)
