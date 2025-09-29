package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// FROM registry.default.svc.cluster.local:5000/autograder-base:ubuntu-22.04
const DOCKERFILE = `
FROM registry.default.svc.cluster.local:5000/autograder-base:ubuntu-22.04
COPY setup.sh /autograder/source/
COPY run_autograder /autograder/source/
COPY run_autograder /autograder/
RUN chmod +x /autograder/run_autograder
RUN chmod +x /autograder/source/setup.sh
RUN bash /autograder/source/setup.sh
`
const BUILDKIT_CONFIG = `[registry.\"registry.default.svc.cluster.local:5000\"]
  http=true
`

func main() {
	const PORT = 5000
	var config *rest.Config
	var err error
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		fmt.Println("Using kubeconfig: ", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Failed to load kubeconfig: %v", err)
		}
	} else {
		fmt.Println("Using in-cluster config")
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Failed to load in-cluster config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes Client: %v", err)
	}

	// ---- Handlers ----

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Only GET method allowed", http.StatusMethodNotAllowed)
			return
		}
		fmt.Fprintln(w, "(.0.2.4) Job Server is running. Send POST requests to /submit")
	})

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
			return
		}

	})

	http.HandleFunc("/configure", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		assignment := strings.ToLower(r.FormValue("assignment"))
		if assignment == "" {
			http.Error(w, "Missing 'assignment' field", http.StatusBadRequest)
			return
		}
		zipFile, _, err := r.FormFile("setup")
		if err != nil {
			http.Error(w, "missing zip File", http.StatusBadRequest)
			return
		}
		defer zipFile.Close()
		zipData, err := io.ReadAll(zipFile)
		encodedZipData := base64.StdEncoding.EncodeToString(zipData)

		// ag_script, _, err := r.FormFile("run_autograder")
		// if err != nil {
		// 	http.Error(w, "missing required script: run_autograder", http.StatusBadRequest)
		// 	return
		// }
		// defer ag_script.Close()

		// autograderData, err := io.ReadAll(ag_script)
		// if err != nil {
		// 	http.Error(w, "Failed to read run_autograder", http.StatusInternalServerError)
		// 	return
		// }

		// setup_script, _, err := r.FormFile("setup")
		// if err != nil {
		// 	http.Error(w, "missing required script: setup.sh", http.StatusBadRequest)
		// 	return
		// }
		// setupData, err := io.ReadAll(setup_script)
		// if err != nil {
		// 	http.Error(w, "Failed to read setup.sh", http.StatusInternalServerError)
		// 	return
		// }

		// //Create ConfigMap for scripts (setup.sh and run_autograder)
		// configMapName := "script-cm-" + assignment + fmt.Sprintf("-%d", time.Now().Unix())

		// _, err = clientset.CoreV1().ConfigMaps("default").Create(context.TODO(), &corev1.ConfigMap{
		// 	ObjectMeta: meta.ObjectMeta{
		// 		Name: configMapName,
		// 	},
		// 	Data: map[string]string{
		// 		"setup.sh":       string(setupData),
		// 		"run_autograder": string(autograderData),
		// 		"Dockerfile":     DOCKERFILE,
		// 	},
		// }, meta.CreateOptions{})
		// if err != nil {
		// 	http.Error(w, fmt.Sprintf("Failed to create ConfigMap: %v", err), http.StatusInternalServerError)
		// 	return
		// }

		//Create job to create assignment image
		buildkitPrivileged := true
		builderClient := clientset.BatchV1().Jobs("default")
		buildJobName := fmt.Sprintf("build-image-%s-%d", assignment, time.Now().Unix())
		builderJob_ttl := int32(60) //Keep builder job for 1 minute after completion

		builderJob := &batchv1.Job{
			ObjectMeta: meta.ObjectMeta{
				Name: buildJobName,
			},
			Spec: batchv1.JobSpec{
				TTLSecondsAfterFinished: &builderJob_ttl,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Volumes: []corev1.Volume{
							{
								Name: "workspace-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						InitContainers: []corev1.Container{
							{
								Name:            "buildkit-setup",
								Image:           "alpine/git",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         []string{"/bin/sh", "-c"},
								Args: []string{
									fmt.Sprintf("echo \"%s\" | base64 -d > /tmp/files.zip && unzip /tmp/files.zip -d /workspace && echo \"%s\" > /workspace/Dockerfile", encodedZipData, DOCKERFILE),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "workspace-volume",
										MountPath: "/workspace",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "buildkit",
								Image: "moby/buildkit:latest",
								SecurityContext: &corev1.SecurityContext{
									Privileged: &buildkitPrivileged,
								},
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         []string{"sh", "-c"},
								Args: []string{
									fmt.Sprintf(
										"printf '%%s' \"%s\" > /etc/buildkit/buildkitd.toml && exec buildctl-daemonless.sh build --frontend dockerfile.v0 --local context=/workspace --local dockerfile=/workspace --output type=image,name=registry.default.svc.cluster.local:5000/assignment:%s,push=true",
										BUILDKIT_CONFIG,
										assignment,
									),
								},
								Env: []corev1.EnvVar{
									{
										Name:  "BUILDKITD_FLAGS",
										Value: "--allow-insecure-entitlement network.host",
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "workspace-volume",
										MountPath: "/workspace",
									},
								},
							},
						},
					},
				},
			},
		}

		_, err = builderClient.Create(context.TODO(), builderJob, meta.CreateOptions{})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create builder Job: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"jobName": buildJobName,
			"status":  "created",
		})

	})

	log.Printf("Server listening on :%d\n", PORT)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", PORT), nil))
}

// Helper Functions
