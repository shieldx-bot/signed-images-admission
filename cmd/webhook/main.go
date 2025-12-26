package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	k8s "github.com/shieldx-bot/signed-images-admission/pkg/k8s"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	// Core logic verify chữ ký cosign
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/signature"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

type admissionReview struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Request    *struct {
		UID    string "json:\"uid\""
		Object struct {
			ApiServer string "json:\"apiServer,omitempty\""
			Kind      string "json:\"kind,omitempty\""
			Metadata  struct {
				Name      string `json:"name,omitempty"`
				Namespace string `json:"namespace,omitempty"`
			}
			Spec struct {
				Containers []struct {
					Name  string `json:"name,omitempty"`
					Image string `json:"image,omitempty"`
				}
			}
		}
	}
}

// admissionResponse là AdmissionReview response trả về cho kube-apiserver.
//
// IMPORTANT: Trong Go, encoding/json CHỈ marshal các field được export (viết hoa).
// Nếu field là `response` / `status` (viết thường) thì JSON output sẽ bị mất phần đó.
// Đây là lý do bạn chỉ thấy {apiVersion, kind, image}.
type admissionResponse struct {
	APIVersion string                `json:"apiVersion"`
	Kind       string                `json:"kind"`
	Image      string                `json:"image,omitempty"`
	Response   admissionResponseBody `json:"response"`
}

type admissionResponseBody struct {
	UID     string          `json:"uid"`
	Allowed bool            `json:"allowed"`
	Status  admissionStatus `json:"status,omitempty"`
}

type admissionStatus struct {
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Code    int32  `json:"code,omitempty"`
}

func userFacingVerifyMessage(image string, err error) admissionStatus {
	// Keep this message short and actionable because kubectl prints it directly.
	// We still include a hint so new users know what to do next.
	msg := fmt.Sprintf("Denied: image signature verification failed for %q.", image)
	status := admissionStatus{
		Message: msg,
		Reason:  "ImageSignatureVerificationFailed",
		Code:    403,
	}
	if err == nil {
		return status
	}

	// Add a compact hint depending on common failure modes.
	es := err.Error()
	switch {
	case strings.Contains(es, ".sig") && strings.Contains(es, "signature artifact"):
		status.Message = msg + " Use the real image (repo:tag or repo@sha256:...), not the cosign .sig artifact tag."
	case strings.Contains(es, "no matching signatures") || strings.Contains(es, "no signatures"):
		status.Message = msg + " The image is not signed with the expected key. Sign it with cosign and redeploy."
	case strings.Contains(es, "load public key"):
		status.Message = msg + " The webhook cannot read COSIGN_PUB_KEY. Ensure the public key is mounted and the env var is correct."
	case strings.Contains(es, "cannot load Rekor public keys"):
		status.Message = msg + " Rekor keys are unavailable. Fix tlog trust, or (dev only) set COSIGN_IGNORE_TLOG=true."
	default:
		// Keep the technical error, but not too verbose.
		status.Message = msg + " " + es
	}

	return status
}

func VerifyImageSignature(image string) error {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	img := strings.TrimSpace(image)
	if img == "" {
		return fmt.Errorf("empty image")
	}

	if strings.HasSuffix(img, ".sig") && strings.Contains(img, ":sha256-") {
		return fmt.Errorf("image looks like a cosign signature artifact tag (ends with .sig); verify the real image tag/digest instead, e.g. repo:tag or repo@sha256:...")
	}
	ref, err := name.ParseReference(image)
	if err != nil {
		return err
	}

	pubKeyPath := getenv("COSIGN_PUB_KEY", "./cosign.pub")
	verifier, err := signature.LoadPublicKey(ctx, pubKeyPath)
	if err != nil {
		return fmt.Errorf("load public key %q: %w", pubKeyPath, err)
	}

	co := &cosign.CheckOpts{SigVerifier: verifier}

	if rekorPubs, e := cosign.GetRekorPubs(ctx); e == nil {
		co.RekorPubKeys = rekorPubs
	} else {
		if getenv("COSIGN_IGNORE_TLOG", "false") == "true" {
			co.IgnoreTlog = true
			log.Printf("warning: cannot load Rekor public keys (%v); COSIGN_IGNORE_TLOG=true so skipping tlog verification", e)
		} else {
			return fmt.Errorf("cannot load Rekor public keys (needed to verify bundle): %w (set COSIGN_IGNORE_TLOG=true to skip tlog verification)", e)
		}
	}

	_, _, err = cosign.VerifyImageSignatures(ctx, ref, co)
	if err != nil {
		return fmt.Errorf("verify failed for %q: %w", img, err)
	}
	return nil

}
func main() {
	r := gin.Default()

	// CORS is only relevant for browser-based clients (the dashboard UI).
	// Admission calls from kube-apiserver do NOT require CORS.
	//
	// Configure allowed origins via env (comma-separated), for example:
	//   CORS_ALLOW_ORIGINS=https://frontend.example.com,http://localhost:5173
	allowedOrigins := splitCSV(getenv("CORS_ALLOW_ORIGINS", "https://frontend.example.com,http://localhost:5173"))
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"https://frontend.example.com", "http://localhost:5173", "https://frontend-signed.security.svc.cluster.local", "https://frontend-signed.security"}
	}
	allowAll := strings.EqualFold(strings.TrimSpace(getenv("CORS_ALLOW_ALL", "false")), "true")
	if allowAll {
		r.Use(cors.New(cors.Config{
			AllowAllOrigins: true,
			AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
			ExposeHeaders:   []string{"Content-Length"},
			MaxAge:          12 * time.Hour,
		}))
	} else {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     allowedOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}))
	}

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong v1.0.0"})
	})

	r.GET("/ListNameSpace", func(c *gin.Context) {
		clientset, err := k8s.GetClientset()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		nsList, err := clientset.CoreV1().Namespaces().List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		names := make([]string, 0, len(nsList.Items))
		for _, ns := range nsList.Items {
			names = append(names, ns.Name)
		}
		c.JSON(http.StatusOK, gin.H{"namespaces": names})
	})
	// Ảnh 1: "The Core Logic" (Logic cốt lõi)
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
		if len(review.Request.Object.Spec.Containers) == 0 {
			c.JSON(http.StatusOK, admissionResponse{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
				Response: admissionResponseBody{
					UID:     review.Request.UID,
					Allowed: false,
					Status:  admissionStatus{Message: "no containers found in pod spec"},
				},
			})
			return
		}

		image := review.Request.Object.Spec.Containers[0].Image
		log.Printf("admission uid=%s pod=%s/%s image=%q", review.Request.UID, review.Request.Object.Metadata.Namespace, review.Request.Object.Metadata.Name, image)
		err := VerifyImageSignature(image)
		// Xử lý kết quả verify ở đây
		if err != nil {
			log.Printf("verify FAILED: uid=%s image=%q err=%v", review.Request.UID, image, err)
			resp := admissionResponse{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			}
			resp.Image = image
			resp.Response.UID = review.Request.UID
			resp.Response.Allowed = false // Mặc định không cho phép
			resp.Response.Status = userFacingVerifyMessage(image, err)
			res, err := json.MarshalIndent(resp, "", "  ")
			if err == nil {
				telegramToken := getenv("TELEGRAM_TOKEN", "8526833134:AAHKYZ_tXx_gKj6JeAEToby8F7BiRblKwrc")
				telegramChatID := getenv("TELEGRAM_CHAT_ID", "-5090601314")
				if telegramToken != "" && telegramChatID != "" {
					msg := fmt.Sprintf("New admission review processed:\n```\n%s\n```", string(res))
					http.PostForm(
						fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramToken),
						map[string][]string{
							"chat_id":    {telegramChatID},
							"text":       {msg},
							"parse_mode": {"Markdown"},
						},
					)
				}
			}
			c.JSON(http.StatusOK, resp)
			// Ảnh 1: "The Core Logic" (Logic cốt lõi)
			// c.JSON(http.StatusOK, resp) When error occurs, send response here
		} else {
			log.Printf("verify OK: uid=%s image=%q", review.Request.UID, image)
			resp := admissionResponse{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			}
			resp.Image = image
			resp.Response.UID = review.Request.UID
			resp.Response.Allowed = true // Mặc định cho phép
			var msg string = " All images verified successfully."
			resp.Response.Status.Message = msg
			res, err := json.MarshalIndent(resp, "", "  ")
			if err == nil {
				telegramToken := getenv("TELEGRAM_TOKEN", "8526833134:AAHKYZ_tXx_gKj6JeAEToby8F7BiRblKwrc")
				telegramChatID := getenv("TELEGRAM_CHAT_ID", "-5090601314")
				if telegramToken != "" && telegramChatID != "" {
					msg := fmt.Sprintf("New admission review processed:\n```\n%s\n```", string(res))
					http.PostForm(
						fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramToken),
						map[string][]string{
							"chat_id":    {telegramChatID},
							"text":       {msg},
							"parse_mode": {"Markdown"},
						},
					)
				}
			}
			c.JSON(http.StatusOK, resp)

		}

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
