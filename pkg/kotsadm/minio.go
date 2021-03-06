package kotsadm

import (
	"bytes"

	"github.com/pkg/errors"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

func getMinioYAML(namespace string) (map[string][]byte, error) {
	docs := map[string][]byte{}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var configMap bytes.Buffer
	if err := s.Encode(minioConfigMap(namespace), &configMap); err != nil {
		return nil, errors.Wrap(err, "failed to marshal minio config map")
	}
	docs["minio-configmap.yaml"] = configMap.Bytes()

	var statefulset bytes.Buffer
	if err := s.Encode(minioStatefulset(namespace), &statefulset); err != nil {
		return nil, errors.Wrap(err, "failed to marshal minio statefulset")
	}
	docs["minio-statefulset.yaml"] = statefulset.Bytes()

	var service bytes.Buffer
	if err := s.Encode(minioService(namespace), &service); err != nil {
		return nil, errors.Wrap(err, "failed to marshal minio service")
	}
	docs["minio-service.yaml"] = service.Bytes()

	var job bytes.Buffer
	if err := s.Encode(minioJob(namespace), &job); err != nil {
		return nil, errors.Wrap(err, "failed to marshal minio job")
	}
	docs["minio-job.yaml"] = job.Bytes()

	return docs, nil
}

func ensureMinio(deployOptions DeployOptions, clientset *kubernetes.Clientset) error {
	if err := ensureMinioConfigMap(deployOptions.Namespace, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure minio configmap")
	}

	if err := ensureMinioStatefulset(deployOptions.Namespace, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure minio statefulset")
	}

	if err := ensureMinioService(deployOptions.Namespace, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure minio service")
	}

	if err := ensureMinioJob(deployOptions.Namespace, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure minio job")
	}

	return nil
}

func ensureMinioConfigMap(namespace string, clientset *kubernetes.Clientset) error {
	_, err := clientset.CoreV1().ConfigMaps(namespace).Get("kotsadm-minio", metav1.GetOptions{})
	if err != nil {
		if !kuberneteserrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get existing config map")
		}

		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(minioConfigMap(namespace))
		if err != nil {
			return errors.Wrap(err, "failed to create configmap")
		}
	}

	return nil
}

func ensureMinioStatefulset(namespace string, clientset *kubernetes.Clientset) error {
	_, err := clientset.AppsV1().StatefulSets(namespace).Get("kotsadm-minio", metav1.GetOptions{})
	if err != nil {
		if !kuberneteserrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get existing statefulset")
		}

		_, err := clientset.AppsV1().StatefulSets(namespace).Create(minioStatefulset(namespace))
		if err != nil {
			return errors.Wrap(err, "failed to create minio statefulset")
		}
	}

	return nil
}

func ensureMinioService(namespace string, clientset *kubernetes.Clientset) error {
	_, err := clientset.CoreV1().Services(namespace).Get("kotsadm-minio", metav1.GetOptions{})
	if err != nil {
		if !kuberneteserrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get existing service")
		}

		_, err := clientset.CoreV1().Services(namespace).Create(minioService(namespace))
		if err != nil {
			return errors.Wrap(err, "failed to create service")
		}
	}

	return nil
}

func ensureMinioJob(namespace string, clientset *kubernetes.Clientset) error {
	_, err := clientset.BatchV1().Jobs(namespace).Get("kotsadm-minio", metav1.GetOptions{})
	if err != nil {
		if !kuberneteserrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get existing job")
		}

		_, err := clientset.BatchV1().Jobs(namespace).Create(minioJob(namespace))
		if err != nil {
			return errors.Wrap(err, "failed to create job")
		}
	}

	return nil
}
