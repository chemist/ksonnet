package metadata

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/spf13/afero"
)

const (
	blankSwagger     = "/blankSwagger.json"
	blankSwaggerData = `{
  "swagger": "2.0",
  "info": {
   "title": "Kubernetes",
   "version": "v1.7.0"
  },
  "paths": {
  },
  "definitions": {
  }
}`
	blankK8sLib = `// AUTOGENERATED from the Kubernetes OpenAPI specification. DO NOT MODIFY.
// Kubernetes version: v1.7.0

{
  local hidden = {
  },
}
`
)

var testFS = afero.NewMemMapFs()

func init() {
	afero.WriteFile(testFS, blankSwagger, []byte(blankSwaggerData), os.ModePerm)
}

func TestInitSuccess(t *testing.T) {
	spec, err := parseClusterSpec(fmt.Sprintf("file:%s", blankSwagger), testFS)
	if err != nil {
		t.Fatalf("Failed to parse cluster spec: %v", err)
	}

	appPath := AbsPath("/fromEmptySwagger")
	_, err = initManager(appPath, spec, testFS)
	if err != nil {
		t.Fatalf("Failed to init cluster spec: %v", err)
	}

	defaultEnvDir := appendToAbsPath(environmentsDir, defaultEnvName)
	paths := []AbsPath{
		ksonnetDir,
		libDir,
		componentsDir,
		environmentsDir,
		vendorDir,
		defaultEnvDir,
	}

	for _, p := range paths {
		path := appendToAbsPath(appPath, string(p))
		exists, err := afero.DirExists(testFS, string(path))
		if err != nil {
			t.Fatalf("Expected to create directory '%s', but failed:\n%v", p, err)
		} else if !exists {
			t.Fatalf("Expected to create directory '%s', but failed", path)
		}
	}

	envPath := appendToAbsPath(appPath, string(defaultEnvDir))
	schemaPath := appendToAbsPath(envPath, schemaFilename)
	bytes, err := afero.ReadFile(testFS, string(schemaPath))
	if err != nil {
		t.Fatalf("Failed to read swagger file at '%s':\n%v", schemaPath, err)
	} else if actualSwagger := string(bytes); actualSwagger != blankSwaggerData {
		t.Fatalf("Expected swagger file at '%s' to have value: '%s', got: '%s'", schemaPath, blankSwaggerData, actualSwagger)
	}

	k8sLibPath := appendToAbsPath(envPath, k8sLibFilename)
	k8sLibBytes, err := afero.ReadFile(testFS, string(k8sLibPath))
	if err != nil {
		t.Fatalf("Failed to read ksonnet-lib file at '%s':\n%v", k8sLibPath, err)
	} else if actualK8sLib := string(k8sLibBytes); actualK8sLib != blankK8sLib {
		t.Fatalf("Expected swagger file at '%s' to have value: '%s', got: '%s'", k8sLibPath, blankK8sLib, actualK8sLib)
	}

	extensionsLibPath := appendToAbsPath(envPath, extensionsLibFilename)
	extensionsLibBytes, err := afero.ReadFile(testFS, string(extensionsLibPath))
	if err != nil {
		t.Fatalf("Failed to read ksonnet-lib file at '%s':\n%v", extensionsLibPath, err)
	} else if string(extensionsLibBytes) == "" {
		t.Fatalf("Expected extension library file at '%s' to be non-empty", extensionsLibPath)
	}
}

func TestFindSuccess(t *testing.T) {
	findSuccess := func(t *testing.T, appDir, currDir AbsPath) {
		m, err := findManager(currDir, testFS)
		if err != nil {
			t.Fatalf("Failed to find manager at path '%s':\n%v", currDir, err)
		} else if m.rootPath != appDir {
			t.Fatalf("Found manager at incorrect path '%s', expected '%s'", m.rootPath, appDir)
		}
	}

	spec, err := parseClusterSpec(fmt.Sprintf("file:%s", blankSwagger), testFS)
	if err != nil {
		t.Fatalf("Failed to parse cluster spec: %v", err)
	}

	appPath := AbsPath("/findSuccess")
	_, err = initManager(appPath, spec, testFS)
	if err != nil {
		t.Fatalf("Failed to init cluster spec: %v", err)
	}

	findSuccess(t, appPath, appPath)

	components := appendToAbsPath(appPath, componentsDir)
	findSuccess(t, appPath, components)

	// Create empty app file.
	appFile := appendToAbsPath(components, "app.jsonnet")
	f, err := testFS.OpenFile(string(appFile), os.O_RDONLY|os.O_CREATE, 0777)
	if err != nil {
		t.Fatalf("Failed to touch app file '%s'\n%v", appFile, err)
	}
	f.Close()

	findSuccess(t, appPath, appFile)
}

func TestComponentPaths(t *testing.T) {
	spec, err := parseClusterSpec(fmt.Sprintf("file:%s", blankSwagger), testFS)
	if err != nil {
		t.Fatalf("Failed to parse cluster spec: %v", err)
	}

	appPath := AbsPath("/componentPaths")
	m, err := initManager(appPath, spec, testFS)
	if err != nil {
		t.Fatalf("Failed to init cluster spec: %v", err)
	}

	// Create empty app file.
	components := appendToAbsPath(appPath, componentsDir)
	appFile1 := appendToAbsPath(components, "component1.jsonnet")
	f1, err := testFS.OpenFile(string(appFile1), os.O_RDONLY|os.O_CREATE, 0777)
	if err != nil {
		t.Fatalf("Failed to touch app file '%s'\n%v", appFile1, err)
	}
	f1.Close()

	// Create empty file in a nested directory.
	appSubdir := appendToAbsPath(components, "appSubdir")
	err = testFS.MkdirAll(string(appSubdir), os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create directory '%s'\n%v", appSubdir, err)
	}
	appFile2 := appendToAbsPath(appSubdir, "component2.jsonnet")
	f2, err := testFS.OpenFile(string(appFile2), os.O_RDONLY|os.O_CREATE, 0777)
	if err != nil {
		t.Fatalf("Failed to touch app file '%s'\n%v", appFile1, err)
	}
	f2.Close()

	// Create a directory that won't be listed in the call to `ComponentPaths`.
	unlistedDir := string(appendToAbsPath(components, "doNotListMe"))
	err = testFS.MkdirAll(unlistedDir, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create directory '%s'\n%v", unlistedDir, err)
	}

	paths, err := m.ComponentPaths()
	if err != nil {
		t.Fatalf("Failed to find component paths: %v", err)
	}

	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })

	if len(paths) != 2 || paths[0] != string(appFile2) || paths[1] != string(appFile1) {
		t.Fatalf("m.ComponentPaths failed; expected '%s', got '%s'", []string{string(appFile1), string(appFile2)}, paths)
	}
}

func TestFindFailure(t *testing.T) {
	findFailure := func(t *testing.T, currDir AbsPath) {
		_, err := findManager(currDir, testFS)
		if err == nil {
			t.Fatalf("Expected to fail to find ksonnet app in '%s', but succeeded", currDir)
		}
	}

	findFailure(t, "/")
	findFailure(t, "/fakePath")
	findFailure(t, "")
}

func TestDoubleNewFailure(t *testing.T) {
	spec, err := parseClusterSpec(fmt.Sprintf("file:%s", blankSwagger), testFS)
	if err != nil {
		t.Fatalf("Failed to parse cluster spec: %v", err)
	}

	appPath := AbsPath("/doubleNew")

	_, err = initManager(appPath, spec, testFS)
	if err != nil {
		t.Fatalf("Failed to init cluster spec: %v", err)
	}

	targetErr := fmt.Sprintf("Could not create app; directory '%s' already exists", appPath)
	_, err = initManager(appPath, spec, testFS)
	if err == nil || err.Error() != targetErr {
		t.Fatalf("Expected to fail to create app with message '%s', got '%s'", targetErr, err.Error())
	}
}
