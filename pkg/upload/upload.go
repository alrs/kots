package upload

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kots/pkg/k8sutil"
	"github.com/replicatedhq/kots/pkg/logger"
	"github.com/replicatedhq/kots/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type UploadOptions struct {
	Namespace       string
	UpstreamURI     string
	Kubeconfig      string
	ExistingAppSlug string
	NewAppName      string
	VersionLabel    string
	UpdateCursor    string
	License         *string
}

func Upload(path string, uploadOptions UploadOptions) error {
	license, err := findLicense(path)
	if err != nil {
		return errors.Wrap(err, "failed to find license")
	}
	uploadOptions.License = license

	updateCursor, err := findUpdateCursor(path)
	if err != nil {
		return errors.Wrap(err, "failed to find update cursor")
	}

	if updateCursor == "" {
		return errors.New("no update cursor found. this is not yet supported")
	}

	uploadOptions.UpdateCursor = updateCursor

	archiveFilename, err := createUploadableArchive(path)
	if err != nil {
		return errors.Wrap(err, "failed to create uploadable archive")
	}

	defer os.Remove(archiveFilename)

	// Make sure we have a name or slug
	if uploadOptions.ExistingAppSlug == "" && uploadOptions.NewAppName == "" {
		split := strings.Split(path, string(os.PathSeparator))
		lastPathPart := ""
		idx := 1
		for lastPathPart == "" {
			lastPathPart = split[len(split)-idx]
			if lastPathPart == "" && len(split) > idx {
				idx++
				continue
			}

			break
		}

		appName, err := relentlesslyPromptForAppName(lastPathPart)
		if err != nil {
			return errors.Wrap(err, "failed to prompt for app name")
		}

		uploadOptions.NewAppName = appName
	}

	// Make sure we have an upstream URI
	if uploadOptions.ExistingAppSlug == "" && uploadOptions.UpstreamURI == "" {
		upstreamURI, err := promptForUpstreamURI()
		if err != nil {
			return errors.Wrap(err, "failed to prompt for upstream uri")
		}

		uploadOptions.UpstreamURI = upstreamURI
	}

	// Find the kotadm-api pod
	log := logger.NewLogger()
	log.ActionWithSpinner("Uploading local application to Admin Console")

	podName, err := findKotsadm(uploadOptions.Namespace)
	if err != nil {
		log.FinishSpinnerWithError()
		return errors.Wrap(err, "failed to find kotsadm pod")
	}

	// set up port forwarding to get to it
	stopCh, err := k8sutil.PortForward(uploadOptions.Kubeconfig, 3000, 3000, uploadOptions.Namespace, podName)
	if err != nil {
		log.FinishSpinnerWithError()
		return errors.Wrap(err, "failed to start port forwarding")
	}
	defer close(stopCh)

	// upload using http to the pod directly
	req, err := createUploadRequest(archiveFilename, uploadOptions, "http://localhost:3000/api/v1/kots")
	if err != nil {
		log.FinishSpinnerWithError()
		return errors.Wrap(err, "failed to create upload request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.FinishSpinnerWithError()
		return errors.Wrap(err, "failed to execute request")
	}

	if resp.StatusCode != 200 {
		log.FinishSpinnerWithError()
		return errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.FinishSpinnerWithError()
		return errors.Wrap(err, "failed to read response body")
	}
	type UploadResponse struct {
		URI string `json:"uri"`
	}
	var uploadResponse UploadResponse
	if err := json.Unmarshal(b, &uploadResponse); err != nil {
		log.FinishSpinnerWithError()
		return errors.Wrap(err, "failed to unmarshal response")
	}

	log.FinishSpinner()

	return nil
}

func findKotsadm(namespace string) (string, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return "", errors.Wrap(err, "failed to get cluster config")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", errors.Wrap(err, "failed to create kubernetes clientset")
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "app=kotsadm-api"})
	if err != nil {
		return "", errors.Wrap(err, "failed to list pods")
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}

	return "", errors.New("unable to find kotsadm pod")
}

func createUploadRequest(path string, uploadOptions UploadOptions, uri string) (*http.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	archivePart, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create form file")
	}
	_, err = io.Copy(archivePart, file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy file to upload")
	}

	method := ""
	if uploadOptions.ExistingAppSlug != "" {
		method = "PUT"
		metadata := map[string]string{
			"slug":         uploadOptions.ExistingAppSlug,
			"versionLabel": uploadOptions.VersionLabel,
			"updateCursor": uploadOptions.UpdateCursor,
		}
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal json")
		}
		metadataPart, err := writer.CreateFormField("metadata")
		if err != nil {
			return nil, errors.Wrap(err, "failed to add metadata")
		}
		if _, err := io.Copy(metadataPart, bytes.NewReader(b)); err != nil {
			return nil, errors.Wrap(err, "failed to copy metadata")
		}
	} else {
		method = "POST"

		body := map[string]string{
			"name":         uploadOptions.NewAppName,
			"versionLabel": uploadOptions.VersionLabel,
			"upstreamURI":  uploadOptions.UpstreamURI,
			"updateCursor": uploadOptions.UpdateCursor,
		}

		if uploadOptions.License != nil {
			body["license"] = *uploadOptions.License
		}

		b, err := json.Marshal(body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal json")
		}
		metadataPart, err := writer.CreateFormField("metadata")
		if err != nil {
			return nil, errors.Wrap(err, "failed to add metadata")
		}
		if _, err := io.Copy(metadataPart, bytes.NewReader(b)); err != nil {
			return nil, errors.Wrap(err, "failed to copy metadata")
		}
	}

	err = writer.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close writer")
	}

	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new request")
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func relentlesslyPromptForAppName(defaultAppName string) (string, error) {
	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . | bold }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     "Application name:",
		Templates: templates,
		Default:   defaultAppName,
		Validate: func(input string) error {
			if len(input) < 3 {
				return errors.New("invalid app name")
			}
			return nil
		},
	}

	for {
		result, err := prompt.Run()
		if err != nil {
			if err == promptui.ErrInterrupt {
				os.Exit(-1)
			}
			continue
		}

		return result, nil
	}
}

func promptForUpstreamURI() (string, error) {
	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . | bold }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	supportedSchemes := map[string]interface{}{
		"helm":       nil,
		"replicated": nil,
	}

	prompt := promptui.Prompt{
		Label:     "Upstream URI:",
		Templates: templates,
		Validate: func(input string) error {
			if !util.IsURL(input) {
				return errors.New("Please enter a URL")
			}

			u, err := url.ParseRequestURI(input)
			if err != nil {
				return errors.New("Invalid URL")
			}

			_, ok := supportedSchemes[u.Scheme]
			if !ok {
				return errors.New("Unsupported upstream type")
			}

			return nil
		},
	}

	for {
		result, err := prompt.Run()
		if err != nil {
			if err == promptui.ErrInterrupt {
				os.Exit(-1)
			}
			continue
		}

		return result, nil
	}
}
