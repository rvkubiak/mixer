package testenv

import (
	"flag"
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	mixerapi "istio.io/api/mixer/v1"
	"istio.io/mixer/adapter/denier"
	"istio.io/mixer/pkg/adapter"
	"istio.io/mixer/pkg/attribute"
	"istio.io/mixer/template"
)

func closeHelper(c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func copyFile(dest, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer closeHelper(in)
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer closeHelper(out)
	_, err = io.Copy(out, in)
	return err

}

func buildConfigStore(relativePaths []string) (string, error) {
	currentPath, err := os.Getwd()
	if err != nil {
		return "", err
	}

	configPath := path.Join(currentPath, uuid.New())
	if err = os.Mkdir(configPath, os.ModePerm); err != nil {
		return "", err
	}

	for _, filePath := range relativePaths {
		err = copyFile(path.Join(configPath, path.Base(filePath)), path.Join(currentPath, filePath))
		if err != nil {
			return "", err
		}
	}

	return configPath, nil
}

func getAttrBag(attribs map[string]interface{}) mixerapi.Attributes {
	requestBag := attribute.GetMutableBag(nil)
	requestBag.Set("target.service", "svc.cluster.local")
	for k, v := range attribs {
		requestBag.Set(k, v)
	}

	var attrs mixerapi.Attributes
	requestBag.ToProto(&attrs, nil, 0)
	return attrs
}

func TestMain(m *testing.M) {
	flag.Parse()
	code := m.Run()
	os.Exit(code)
}

func TestDenierAdapter(t *testing.T) {
	configStore, err := buildConfigStore([]string{
		"../../testdata/config/attributes.yaml",
		"../../testdata/config/deny.yaml",
	})
	if err != nil {
		t.Fatalf("fail to build test config store: %v", err)
	}

	defer func() {
		if removeErr := os.RemoveAll(configStore); removeErr != nil {
			log.Fatal(removeErr)
		}
	}()

	var args = Args{
		// Start Mixer server on a free port on loop back interface
		MixerServerAddr:               `127.0.0.1:0`,
		ConfigStoreURL:                `fs://` + configStore,
		ConfigStore2URL:               `fs://` + configStore,
		ConfigDefaultNamespace:        "istio-config-default",
		ConfigIdentityAttributeDomain: "svc.cluster.local",
		UseAstEvaluator:               true,
	}

	env, err := NewEnv(&args, template.SupportedTmplInfo, []adapter.InfoFn{denier.GetInfo})
	if err != nil {
		t.Fatalf("fail to create testenv: %v", err)
	}

	defer closeHelper(env)

	client, conn, err := env.CreateMixerClient()
	if err != nil {
		t.Fatalf("fail to create client connection: %v", err)
	}
	defer closeHelper(conn)

	attribs := map[string]interface{}{"request.headers": map[string]string{"clnt": "abc"}}
	bag := getAttrBag(attribs)
	request := mixerapi.CheckRequest{Attributes: bag}
	resq, err := client.Check(context.Background(), &request)
	if err != nil {
		t.Errorf("fail to send check to Mixer %v", err)
	}
	if resq.Precondition.Status.Code != 7 {
		t.Error(`resq.Precondition.Status.Code != 7`)
	}
}
