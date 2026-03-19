package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestBuildPod_Basic(t *testing.T) {
	pod := buildPod(PodSpec{
		Name:      "ci-test",
		Namespace: "default",
		Image:     "golang:1.23",
		Command:   []string{"go", "test", "./..."},
	})

	if pod.Name != "ci-test" {
		t.Errorf("name = %q, want ci-test", pod.Name)
	}
	if pod.Spec.Containers[0].Image != "golang:1.23" {
		t.Errorf("image = %q, want golang:1.23", pod.Spec.Containers[0].Image)
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Error("restart policy should be Never")
	}
	if pod.Spec.ServiceAccountName != "temporalci-ci-job" {
		t.Errorf("service account = %q, want temporalci-ci-job", pod.Spec.ServiceAccountName)
	}
}

func TestBuildPod_Resources(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		CPU: "2", Memory: "4Gi",
	})

	res := pod.Spec.Containers[0].Resources
	if res.Requests.Cpu().String() != "2" {
		t.Errorf("cpu request = %s, want 2", res.Requests.Cpu().String())
	}
	if res.Limits.Memory().String() != "4Gi" {
		t.Errorf("memory limit = %s, want 4Gi", res.Limits.Memory().String())
	}
}

func TestBuildPod_CIJobsToleration(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		Tolerations:  []string{"ci-jobs"},
		NodeSelector: map[string]string{"workload": "ci-jobs"},
	})

	if len(pod.Spec.Tolerations) != 1 {
		t.Fatalf("expected 1 toleration, got %d", len(pod.Spec.Tolerations))
	}
	tol := pod.Spec.Tolerations[0]
	if tol.Key != "workload" || tol.Value != "ci-job" || tol.Effect != corev1.TaintEffectNoSchedule {
		t.Errorf("toleration = %+v, want workload=ci-job:NoSchedule", tol)
	}
	if pod.Spec.NodeSelector["workload"] != "ci-jobs" {
		t.Errorf("nodeSelector = %v, want workload=ci-jobs", pod.Spec.NodeSelector)
	}
}

func TestBuildPod_CloneInitContainer(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		CloneURL: "https://github.com/test/repo.git",
		CloneRef: "main",
	})

	if len(pod.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 init container, got %d", len(pod.Spec.InitContainers))
	}
	if pod.Spec.InitContainers[0].Name != "clone" {
		t.Errorf("init container name = %q, want clone", pod.Spec.InitContainers[0].Name)
	}
	if pod.Spec.Containers[0].WorkingDir != "/workspace" {
		t.Errorf("workingDir = %q, want /workspace", pod.Spec.Containers[0].WorkingDir)
	}
}

func TestBuildPod_PVCCache(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		CachePVC: "go-cache-pvc",
	})

	found := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "go-cache" {
			if v.PersistentVolumeClaim == nil {
				t.Error("go-cache volume should use PVC")
			} else if v.PersistentVolumeClaim.ClaimName != "go-cache-pvc" {
				t.Errorf("PVC name = %q, want go-cache-pvc", v.PersistentVolumeClaim.ClaimName)
			}
			found = true
		}
	}
	if !found {
		t.Error("go-cache volume not found")
	}
}

func TestBuildPod_EmptyDirCacheDefault(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
	})

	for _, v := range pod.Spec.Volumes {
		if v.Name == "go-cache" {
			if v.EmptyDir == nil {
				t.Error("go-cache should default to emptyDir when no PVC")
			}
			return
		}
	}
	t.Error("go-cache volume not found")
}

func TestBuildPod_ArtifactPVC(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		ArtifactPVC: "artifacts-pvc",
	})

	foundVol := false
	foundMount := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "artifacts" {
			foundVol = true
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "artifacts-pvc" {
				t.Error("artifacts volume should use PVC artifacts-pvc")
			}
		}
	}
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if m.Name == "artifacts" && m.MountPath == "/artifacts" {
			foundMount = true
		}
	}
	if !foundVol {
		t.Error("artifacts volume not found")
	}
	if !foundMount {
		t.Error("artifacts mount not found")
	}
}

func TestBuildPod_NoArtifactByDefault(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
	})

	for _, v := range pod.Spec.Volumes {
		if v.Name == "artifacts" {
			t.Error("artifacts volume should not exist when ArtifactPVC is empty")
		}
	}
}

func TestBuildPod_DockerInDocker(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		Docker: true,
	})

	// Should have 2 containers: ci + dind
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(pod.Spec.Containers))
	}
	dind := pod.Spec.Containers[1]
	if dind.Name != "dind" {
		t.Errorf("sidecar name = %q, want dind", dind.Name)
	}
	if dind.Image != "docker:27-dind" {
		t.Errorf("dind image = %q", dind.Image)
	}
	if dind.SecurityContext == nil || !*dind.SecurityContext.Privileged {
		t.Error("dind should be privileged")
	}

	// Check docker socket volume
	foundSock := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "docker-sock" {
			foundSock = true
		}
	}
	if !foundSock {
		t.Error("docker-sock volume not found")
	}

	// Check DOCKER_HOST env on main container
	foundHost := false
	for _, e := range pod.Spec.Containers[0].Env {
		if e.Name == "DOCKER_HOST" {
			foundHost = true
		}
	}
	if !foundHost {
		t.Error("DOCKER_HOST env not set on main container")
	}
}

func TestBuildPod_ServiceContainers(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		Services: []ServiceSpec{
			{
				Name:  "postgres",
				Image: "postgres:16",
				Ports: []int{5432},
				Health: &HealthSpec{
					Cmd:     "pg_isready",
					Retries: 30,
				},
				Env: map[string]string{"POSTGRES_PASSWORD": "test"},
			},
		},
	})

	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(pod.Spec.Containers))
	}
	svc := pod.Spec.Containers[1]
	if svc.Name != "postgres" {
		t.Errorf("service name = %q", svc.Name)
	}
	if len(svc.Ports) != 1 || svc.Ports[0].ContainerPort != 5432 {
		t.Errorf("ports = %v", svc.Ports)
	}
	if svc.StartupProbe == nil {
		t.Error("startup probe should be set")
	} else if svc.StartupProbe.FailureThreshold != 30 {
		t.Errorf("failure threshold = %d, want 30", svc.StartupProbe.FailureThreshold)
	}
}

func TestBuildPod_Privileged(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		Privileged: true,
	})

	sc := pod.Spec.Containers[0].SecurityContext
	if sc == nil || !*sc.Privileged {
		t.Error("container should be privileged")
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Add) == 0 {
		t.Error("SYS_ADMIN capability should be added")
	}
}

func TestBuildPod_CollectOutputs(t *testing.T) {
	pod := buildPod(PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.23", Command: []string{"go", "test"},
		CollectOutputs: true,
	})

	foundVol := false
	foundMount := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "outputs" && v.EmptyDir != nil {
			foundVol = true
		}
	}
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if m.Name == "outputs" && m.MountPath == "/temporalci/outputs" {
			foundMount = true
		}
	}
	if !foundVol {
		t.Error("outputs volume not found")
	}
	if !foundMount {
		t.Error("outputs mount not found")
	}
}

func TestParseOutputsFromLogs(t *testing.T) {
	logs := `Building...
Done.
::temporalci-outputs-start::
CLUSTER_ENDPOINT=https://my-cluster.example.com
CLUSTER_NAME=integ-42
::temporalci-outputs-end::
Cleanup complete.`

	outputs := parseOutputsFromLogs(logs)
	if outputs["CLUSTER_ENDPOINT"] != "https://my-cluster.example.com" {
		t.Errorf("CLUSTER_ENDPOINT = %q", outputs["CLUSTER_ENDPOINT"])
	}
	if outputs["CLUSTER_NAME"] != "integ-42" {
		t.Errorf("CLUSTER_NAME = %q", outputs["CLUSTER_NAME"])
	}
}

func TestParseOutputsFromLogs_NoMarkers(t *testing.T) {
	outputs := parseOutputsFromLogs("just some logs\nno outputs here")
	if len(outputs) != 0 {
		t.Errorf("expected empty outputs, got %v", outputs)
	}
}
