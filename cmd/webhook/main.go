// package main

// import (
// 	"fmt"
// 	"net/http"

// 	"github.com/gin-gonic/gin"
// )

// func main() {
// 	r := gin.Default()
// 	r.GET("/ping", func(c *gin.Context) {
// 		c.JSON(http.StatusOK, gin.H{"message": "pong"})
// 	})

// 	r.POST("/webhook", func(c *gin.Context) {
// 		fmt.Printf("Webhook received Successfully\n")
// 		c.JSON(http.StatusOK, gin.H{"message": "webhook received"})

// 	})
// 	r.Run(":8085") // listen and serve on

// }

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	// Core logic verify chữ ký cosign
	"github.com/sigstore/cosign/v2/pkg/cosign"
	// Dùng để làm việc với OCI registry (Docker Hub, GHCR, ...)
	"github.com/sigstore/cosign/v2/pkg/oci/remote"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type admissionReview struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Request    *struct {
		UID    string          `json:"uid"`
		Object json.RawMessage `json:"object"`
	} `json:"request,omitempty"`
	Response *struct {
		UID     string `json:"uid"`
		Allowed bool   `json:"allowed"`
		Status  *struct {
			Message string `json:"message,omitempty"`
		} `json:"status,omitempty"`
	} `json:"response,omitempty"`
}

func verifyIamgeSignature(image string) error {
	ctx := cosign.NewContext()
	ref, err := remote.ParseReference(image)
	if err != nil {
		return err
	}
	pubKeyPath := "./cosign.pub"
	verifier, err := cosign.LoadPublicKey(ctx, pubKeyPath)
	if err != nil {
		return err
	}

	_, err = cosign.VerifyImageSignatures(ctx, ref, verifier)
	if err != nil {
		return err
	}
	return nil

}
func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	r.POST("/webhook", func(c *gin.Context) {
		var review admissionReview
		if err := c.ShouldBindJSON(&review); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if review.Request == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing request"})
			return
		}
		// Example: Verify cosign signatures here
		fmt.Printf("Review: %+v\n", review)

		// For now we allow everything; later you can parse PodSpec images from review.Request.Object.Raw.
		fmt.Printf("admission request uid=%s object=%d bytes\n", review.Request.UID, len(review.Request.Object))
		// if verifyIamgeSignature(string(review.Request.Object)) != nil {
		// 	c.JSON(http.StatusForbidden, gin.H{"error": "image signature verification failed"})
		// }
		resp := admissionReview{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
			Response: &struct {
				UID     string `json:"uid"`
				Allowed bool   `json:"allowed"`
				Status  *struct {
					Message string `json:"message,omitempty"`
				} `json:"status,omitempty"`
			}{
				UID:     review.Request.UID,
				Allowed: true,
				Status: &struct {
					Message string `json:"message,omitempty"`
				}{Message: "allowed by webhook"},
			},
		}

		c.JSON(http.StatusOK, resp)
	})

	addr := ":" + getenv("PORT", "8443")
	certFile := getenv("TLS_CERT_FILE", "/tls/tls.crt")
	keyFile := getenv("TLS_KEY_FILE", "/tls/tls.key")
	if getenv("TLS_ENABLED", "true") == "false" {
		_ = r.Run(addr)
		return
	}
	_ = r.RunTLS(addr, certFile, keyFile)

}

//    resp := admissionReview{
//             APIVersion: "admission.k8s.io/v1",
//             Kind:       "AdmissionReview",
//             Response: &struct {
//                 UID     string `json:"uid"`
//                 Allowed bool   `json:"allowed"`
//                 Status  *struct {
//                     Message string `json:"message,omitempty"`
//                 } `json:"status,omitempty"`
//             }{
//                 UID:     review.Request.UID,
//                 Allowed: true, // set false to deny
//                 Status: &struct {
//                     Message string `json:"message,omitempty"`
//                 }{Message: "allowed by webhook"},
//             },
//         }
