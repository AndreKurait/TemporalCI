package k8s

import (
	"testing"
)

// --- Month 12: DinD, service containers, privileged mode, step outputs ---

func TestBuildPod_DockerInDocker(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.24", Command: []string{"sh", "-c", "docker build ."},
		Docker: true,
	}
	pod := buildPod(spec)

	// Should have DinD sidecar
	found := false
	for _, c := range pod.Spec.Containers {
		if c.Name == "dind" {
			found = true
			if c.Image != "docker:27-dind" {
				t.Errorf("dind image = %q", c.Image)
			}
			if c.SecurityContext == nil || c.SecurityContext.Privileged == nil || !*c.SecurityContext.Privileged {
				t.Error("dind should be privileged")
			}
			break
		}
	}
	if !found {
		t.Error("expected dind sidecar container")
	}

	// Main container should have DOCKER_HOST env
	main := pod.Spec.Containers[0]
	dockerHostFound := false
	for _, e := range main.Env {
		if e.Name == "DOCKER_HOST" {
			dockerHostFound = true
			break
		}
	}
	if !dockerHostFound {
		t.Error("main container should have DOCKER_HOST env var")
	}
}

func TestBuildPod_ServiceContainers(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.24", Command: []string{"sh", "-c", "psql -h localhost"},
		Services: []ServiceSpec{
			{
				Name: "postgres", Image: "postgres:16", Ports: []int{5432},
				Health: &HealthSpec{Cmd: "pg_isready", Interval: "5s", Retries: 30},
				Env:    map[string]string{"POSTGRES_PASSWORD": "test"},
			},
		},
	}
	pod := buildPod(spec)

	// Should have service sidecar
	found := false
	for _, c := range pod.Spec.Containers {
		if c.Name == "postgres" {
			found = true
			if c.Image != "postgres:16" {
				t.Errorf("service image = %q", c.Image)
			}
			break
		}
	}
	if !found {
		t.Error("expected postgres sidecar container")
	}
}

func TestBuildPod_Privileged(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "ubuntu:22.04", Command: []string{"sh", "-c", "kind create cluster"},
		Privileged: true,
	}
	pod := buildPod(spec)

	main := pod.Spec.Containers[0]
	if main.SecurityContext == nil {
		t.Fatal("expected security context")
	}
	if !*main.SecurityContext.Privileged {
		t.Error("expected privileged=true")
	}
	foundSysAdmin := false
	for _, cap := range main.SecurityContext.Capabilities.Add {
		if cap == "SYS_ADMIN" {
			foundSysAdmin = true
		}
	}
	if !foundSysAdmin {
		t.Error("expected SYS_ADMIN capability")
	}
}

func TestBuildPod_CollectOutputs(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.24", Command: []string{"sh", "-c", "echo done"},
		CollectOutputs: true,
	}
	pod := buildPod(spec)

	foundVol := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "outputs" {
			foundVol = true
			break
		}
	}
	if !foundVol {
		t.Error("expected outputs volume")
	}

	foundMount := false
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if m.Name == "outputs" && m.MountPath == "/temporalci/outputs" {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Error("expected outputs volume mount at /temporalci/outputs")
	}
}

func TestBuildPod_DockerCachePVC(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.24", Command: []string{"sh", "-c", "docker build ."},
		Docker: true, DockerCachePVC: "docker-cache-pvc",
	}
	pod := buildPod(spec)

	foundPVC := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "docker-cache" && v.PersistentVolumeClaim != nil {
			if v.PersistentVolumeClaim.ClaimName == "docker-cache-pvc" {
				foundPVC = true
			}
		}
	}
	if !foundPVC {
		t.Error("expected docker-cache PVC volume")
	}
}

func TestBuildPod_ArtifactPVC(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "golang:1.24", Command: []string{"sh", "-c", "echo"},
		ArtifactPVC: "artifacts-pvc",
	}
	pod := buildPod(spec)

	foundVol := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == "artifacts" && v.PersistentVolumeClaim != nil {
			foundVol = true
		}
	}
	if !foundVol {
		t.Error("expected artifacts PVC volume")
	}

	foundMount := false
	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if m.Name == "artifacts" && m.MountPath == "/artifacts" {
			foundMount = true
		}
	}
	if !foundMount {
		t.Error("expected artifacts mount at /artifacts")
	}
}

func TestBuildPod_MultipleServices(t *testing.T) {
	spec := PodSpec{
		Name: "ci-test", Namespace: "default",
		Image: "node:22", Command: []string{"sh", "-c", "npm test"},
		Services: []ServiceSpec{
			{Name: "redis", Image: "redis:7", Ports: []int{6379}},
			{Name: "postgres", Image: "postgres:16", Ports: []int{5432}},
		},
	}
	pod := buildPod(spec)

	svcNames := map[string]bool{}
	for _, c := range pod.Spec.Containers {
		if c.Name != "ci" {
			svcNames[c.Name] = true
		}
	}
	if !svcNames["redis"] || !svcNames["postgres"] {
		t.Errorf("expected redis and postgres sidecars, got %v", svcNames)
	}
}
