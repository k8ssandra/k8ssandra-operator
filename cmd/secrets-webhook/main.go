package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/k8ssandra/k8ssandra-operator/cmd/secrets-webhook/admission"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
)

var logger = zap.NewExample()

func main() {
	logConfig := zap.NewProductionConfig()
	lvl := zap.LevelFlag("log-level", zap.InfoLevel, "the log level")
	logConfig.Level.SetLevel(*lvl)
	logger, _ = logConfig.Build()

	// handle our core application
	http.HandleFunc("/mutate-pods", ServeMutatePods)
	http.HandleFunc("/health", ServeHealth)

	// start the server
	// listens to clear text http on port 8080 unless TLS env var is set to "true"
	if os.Getenv("TLS") == "true" {
		cert := "/etc/admission-webhook/tls/tls.crt"
		key := "/etc/admission-webhook/tls/tls.key"
		logger.Info("Listening on port 443...")
		logger.Fatal("%v", zap.Error(http.ListenAndServeTLS(":443", cert, key, nil)))
	} else {
		logger.Info("Listening on port 8080...")
		logger.Info("%v", zap.Error(http.ListenAndServe(":8080", nil)))
	}
}

// ServeHealth returns 200 when things are good
func ServeHealth(w http.ResponseWriter, r *http.Request) {
	log := logger.With(zap.String("uri", r.RequestURI))
	log.Debug("healthy")
	fmt.Fprint(w, "OK")
}

func ServeMutatePods(w http.ResponseWriter, r *http.Request) {
	log := logger.With(zap.String("uri", r.RequestURI))
	log.Info("received mutation request")

	in, err := parseJsonRequest(*r)
	if err != nil {
		log.Error("bad request", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	adm := admission.Admitter{
		Logger:  logger,
		Request: in.Request,
	}

	out, err := adm.MutatePodReview()
	if err != nil {
		e := fmt.Sprintf("unable to generate admission response: %v", err)
		log.Error(e)
		http.Error(w, e, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jout, err := json.Marshal(out)
	if err != nil {
		e := fmt.Sprintf("unable to marshal admission response: %v", err)
		log.Error(e)
		http.Error(w, e, http.StatusInternalServerError)
		return
	}

	log.Debug("sending response", zap.ByteString("response", jout))
	fmt.Fprintf(w, "%s", jout)
}

func parseJsonRequest(r http.Request) (*admissionv1.AdmissionReview, error) {
	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("invalid content-type %v, expected application/json.", contentType)
	}

	bodybuf := new(bytes.Buffer)
	bodybuf.ReadFrom(r.Body)
	body := bodybuf.Bytes()

	if len(body) == 0 {
		return nil, fmt.Errorf("admission request body is empty")
	}

	var a admissionv1.AdmissionReview

	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("unable to parse admission review request: %v", err)
	}

	if a.Request == nil {
		return nil, fmt.Errorf("unable to use admission request with nil Request field")
	}

	return &a, nil
}
