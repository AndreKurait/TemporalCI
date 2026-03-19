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
